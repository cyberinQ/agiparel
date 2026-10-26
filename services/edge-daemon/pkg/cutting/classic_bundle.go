// Author: Sunil Jain, 2026-10-26 — Generate Classic Cut-Bundle profiles only from verified actual cut plies.
package cutting

import (
	"errors"
	"fmt"
	"strings"
)

// ClassicPlySegment is an inclusive run of plies with homogeneous material
// attributes. A change of lot, shade, or other approved physical boundary MUST
// create a separate segment; bundles cannot cross such boundaries.
type ClassicPlySegment struct {
	First int    `json:"first"`
	Last  int    `json:"last"`
	Lot   string `json:"lot"`
	Shade string `json:"shade"`
}

type ClassicMarkerPosition struct {
	Position int      `json:"position"`
	Size     string   `json:"size"`
	Parts    []string `json:"parts"`
}

// ClassicBundleRequest uses ACTUAL accepted ply count. Next* are supplied by
// the owning transaction, which MUST allocate them atomically across all cuts
// of the PO; this pure function is not itself a serial-number allocator.
type ClassicBundleRequest struct {
	OrderID           string                  `json:"order_id"`
	CutNo             int                     `json:"cut_no"`
	ActualPlies       int                     `json:"actual_plies"`
	MarkerPositions   []ClassicMarkerPosition `json:"marker_positions"`
	PlySegments       []ClassicPlySegment     `json:"ply_segments"`
	TargetBundleSize  int                     `json:"target_bundle_size"`
	MinBundleSize     int                     `json:"min_bundle_size"`
	MaxBundleSize     int                     `json:"max_bundle_size"`
	NextBundleNumber  int                     `json:"next_bundle_number"`
	NextGarmentSerial int                     `json:"next_garment_serial"`
}

// ClassicBundleCard is a deterministic proposed card; NOT an issued tag or
// evidence that physical pieces exist. The factory commits it only following
// approval, conservation, tag binding, and the relevant custody gates.
type ClassicBundleCard struct {
	BundleNumber       int    `json:"bundle_number"`
	OrderID            string `json:"order_id"`
	CutNo              int    `json:"cut_no"`
	MarkerPosition     int    `json:"marker_position"`
	Size               string `json:"size"`
	PartCode           string `json:"part_code"`
	PlyGroup           int    `json:"ply_group"`
	PlyFirst           int    `json:"ply_first"`
	PlyLast            int    `json:"ply_last"`
	PieceCount         int    `json:"piece_count"`
	GarmentSerialFirst int    `json:"garment_serial_first"`
	GarmentSerialLast  int    `json:"garment_serial_last"`
	Lot                string `json:"lot"`
	Shade              string `json:"shade"`
}

type ClassicBundleResult struct {
	Cards             []ClassicBundleCard `json:"cards"`
	NextBundleNumber  int                 `json:"next_bundle_number"`
	NextGarmentSerial int                 `json:"next_garment_serial"`
	GarmentCount      int                 `json:"garment_count"`
}

// BalanceConsecutivePlies partitions a homogeneous ply run into consecutive
// bundles. It prefers full target-size groups; if the tail would be orphaned,
// it redistributes that tail within the final groups while respecting min/max.
// For 101 plies, target=max=25, min=13 the result is [25,25,25,13,13].
func BalanceConsecutivePlies(plies, target, minimum, maximum int) ([]int, error) {
	if plies <= 0 || minimum <= 0 || target < minimum || maximum < target {
		return nil, errors.New("ERR_BUNDLE_POLICY: Require plies>0 and 0<min<=target<=max")
	}
	// First find the smallest number of groups satisfying the maximum; no
	// group may be smaller than minimum. Use integer math throughout.
	groups := (plies + target - 1) / target
	if groups*minimum > plies {
		// More generous maximum cannot solve impossible minimum if fewer than
		// ceil(plies/max) groups are already infeasible.
		groups = (plies + maximum - 1) / maximum
		if groups*minimum > plies {
			return nil, fmt.Errorf("ERR_ORPHAN_BUNDLE: %d plies cannot meet size [%d,%d]", plies, minimum, maximum)
		}
	}
	if groups <= 0 {
		return nil, errors.New("ERR_BUNDLE_POLICY: Invalid group count")
	}
	result := make([]int, groups)
	for i := range result {
		result[i] = minimum
	}
	extra := plies - groups*minimum
	// Fill earlier groups first to match the founder's full-bundle-first
	// ergonomic default. Result remains consecutive when assigning ranges.
	for i := 0; i < groups && extra > 0; i++ {
		room := target - result[i]
		if room > extra {
			room = extra
		}
		result[i] += room
		extra -= room
	}
	for i := 0; i < groups && extra > 0; i++ {
		room := maximum - result[i]
		if room > extra {
			room = extra
		}
		result[i] += room
		extra -= room
	}
	if extra != 0 {
		return nil, errors.New("ERR_BUNDLE_POLICY: Impossible partition")
	}
	return result, nil
}

// BuildClassicBundles returns one card per marker-position/part/ply-group.
// Every garment serial is shared across all parts belonging to the same
// marker-position and ply, and differs across marker positions or cuts.
func BuildClassicBundles(req ClassicBundleRequest) (ClassicBundleResult, error) {
	var out ClassicBundleResult
	if strings.TrimSpace(req.OrderID) == "" || req.CutNo <= 0 || req.ActualPlies <= 0 ||
		req.NextBundleNumber <= 0 || req.NextGarmentSerial <= 0 || len(req.MarkerPositions) == 0 {
		return out, errors.New("ERR_CUT_IDENTITY: Required order, cut, actual plies, marker positions and nonzero counters")
	}
	positions := make(map[int]struct{})
	for _, pos := range req.MarkerPositions {
		if pos.Position <= 0 || strings.TrimSpace(pos.Size) == "" || len(pos.Parts) == 0 {
			return out, errors.New("ERR_MARKER_POSITION: Missing position, size or parts")
		}
		if _, found := positions[pos.Position]; found {
			return out, errors.New("ERR_MARKER_POSITION: Duplicate marker position")
		}
		positions[pos.Position] = struct{}{}
		seenParts := make(map[string]struct{})
		for _, part := range pos.Parts {
			if strings.TrimSpace(part) == "" {
				return out, errors.New("ERR_PART: Blank part")
			}
			if _, found := seenParts[part]; found {
				return out, errors.New("ERR_PART: Duplicate part code requires separate marker occurrence identity")
			}
			seenParts[part] = struct{}{}
		}
	}
	if len(req.PlySegments) == 0 {
		return out, errors.New("ERR_PLY_EVIDENCE: Homogeneous ply segments required")
	}
	expect := 1
	for _, seg := range req.PlySegments {
		if seg.First != expect || seg.Last < seg.First || seg.Last > req.ActualPlies ||
			strings.TrimSpace(seg.Lot) == "" || strings.TrimSpace(seg.Shade) == "" {
			return out, errors.New("ERR_PLY_EVIDENCE: Segments must exactly partition actual plies with documented lot and shade")
		}
		expect = seg.Last + 1
	}
	if expect != req.ActualPlies+1 {
		return out, errors.New("ERR_PLY_EVIDENCE: Actual plies are not fully covered")
	}
	out.NextBundleNumber = req.NextBundleNumber
	out.NextGarmentSerial = req.NextGarmentSerial + req.ActualPlies*len(req.MarkerPositions)
	out.GarmentCount = req.ActualPlies * len(req.MarkerPositions)
	if out.NextGarmentSerial <= req.NextGarmentSerial {
		return ClassicBundleResult{}, errors.New("ERR_COUNTER_OVERFLOW")
	}
	for positionIndex, pos := range req.MarkerPositions {
		serialBase := req.NextGarmentSerial + positionIndex*req.ActualPlies
		group := 0
		for _, seg := range req.PlySegments {
			sizes, err := BalanceConsecutivePlies(seg.Last-seg.First+1, req.TargetBundleSize, req.MinBundleSize, req.MaxBundleSize)
			if err != nil {
				return ClassicBundleResult{}, fmt.Errorf("marker %d segment %d-%d: %w", pos.Position, seg.First, seg.Last, err)
			}
			ply := seg.First
			for _, size := range sizes {
				group++
				last := ply + size - 1
				for _, part := range pos.Parts {
					out.Cards = append(out.Cards, ClassicBundleCard{
						BundleNumber: out.NextBundleNumber, OrderID: req.OrderID, CutNo: req.CutNo,
						MarkerPosition: pos.Position, Size: pos.Size, PartCode: part,
						PlyGroup: group, PlyFirst: ply, PlyLast: last, PieceCount: size,
						GarmentSerialFirst: serialBase + ply - 1, GarmentSerialLast: serialBase + last - 1,
						Lot: seg.Lot, Shade: seg.Shade,
					})
					out.NextBundleNumber++
					if out.NextBundleNumber <= 0 {
						return ClassicBundleResult{}, errors.New("ERR_COUNTER_OVERFLOW")
					}
				}
				ply = last + 1
			}
		}
	}
	return out, nil
}
