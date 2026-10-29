// Author: Sunil Jain, CyberinQ LLC
package onboarding

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

type FactoryWorkbook struct {
	TenantID       string            `json:"tenant_id"`
	FactoryName    string            `json:"factory_name"`
	UnitCode       string            `json:"unit_code"`
	ShiftStartHour int               `json:"shift_start_hour"`
	BundleSize     int               `json:"bundle_size"`
	Lines          []LineRow         `json:"lines"`
	Machines       []MachineRow      `json:"machines"`
	Workers        []WorkerRow       `json:"workers"`
	Styles         []StyleRow        `json:"styles"`
	StyleParts     []StylePartRow    `json:"style_parts"`
	BOMRows        []BOMRow          `json:"bom_rows,omitempty"`
	OperationsDAG  []OperationDAGRow `json:"operations_dag"`
	StoreRolls     []StoreRollRow    `json:"store_rolls"`
	WorkOrders     []WorkOrderRow    `json:"work_orders"`
}

type LineRow struct {
	LineID              string  `json:"line_id"`
	DisplayName         string  `json:"display_name"`
	StationsCount       int     `json:"stations_count"`
	TargetDailyCapacity int     `json:"target_daily_capacity"`
	DwellFloorPct       float64 `json:"dwell_floor_pct"`
}

type MachineRow struct {
	MachineID      string `json:"machine_id"`
	LineID         string `json:"line_id"`
	StationID      string `json:"station_id"`
	Name           string `json:"name"`
	CanonicalClass string `json:"canonical_class"`
	CapacityUPH    int    `json:"capacity_uph"`
	Status         string `json:"status"`
}

type WorkerRow struct {
	WorkerID          string  `json:"worker_id"`
	EmployeeCode      string  `json:"employee_code"`
	Name              string  `json:"name"`
	PrimarySkill      string  `json:"primary_skill"`
	SkillGrade        string  `json:"skill_grade"`
	HourlyRateINR     float64 `json:"hourly_rate_inr"`
	AttendanceStatus  string  `json:"attendance_status"`
	AttendanceDate    string  `json:"attendance_date,omitempty"`
	AssignedLineID    string  `json:"assigned_line_id"`
	AssignedStationID string  `json:"assigned_station_id"`
}

type StyleRow struct {
	StyleID      string  `json:"style_id"`
	StyleCode    string  `json:"style_code"`
	Category     string  `json:"category"`
	FitProfile   string  `json:"fit_profile"`
	TargetSAMMin float64 `json:"target_sam_min"`
	FabricSKU    string  `json:"fabric_sku"`
}

type StylePartRow struct {
	StyleCode      string `json:"style_code"`
	PartCode       string `json:"part_code"`
	PartName       string `json:"part_name"`
	SubBundleOrder int    `json:"sub_bundle_order"`
	MaterialSKU    string `json:"material_sku"`
}

// BOMRow is supplied by the factory workbook; source references are declarations,
// not independent approval or a verified marker yield.
type BOMRow struct {
	StyleCode     string  `json:"style_code"`
	MaterialSKU   string  `json:"material_sku"`
	Category      string  `json:"category"`
	QtyPerGarment float64 `json:"qty_per_garment"`
	UOM           string  `json:"uom"`
	SourceRef     string  `json:"source_ref,omitempty"`
}

type OperationDAGRow struct {
	StyleCode      string  `json:"style_code"`
	OperationID    string  `json:"operation_id"`
	OperationName  string  `json:"operation_name"`
	SAMMinutes     float64 `json:"sam_minutes"`
	MachineType    string  `json:"machine_type"`
	SourcePartCode string  `json:"source_part_code"`
	TargetPartCode string  `json:"target_part_code"`
}

type StoreRollRow struct {
	RollID        string  `json:"roll_id"`
	FactorySKU    string  `json:"factory_sku"`
	LengthM       float64 `json:"length_m"`
	WidthCM       float64 `json:"width_cm"`
	ShadeBand     string  `json:"shade_band"`
	BinLocation   string  `json:"bin_location"`
	Status        string  `json:"status"`
	ReservedOrder string  `json:"reserved_order"`
	ReservedCutNo int     `json:"reserved_cut_no"`
}

type WorkOrderRow struct {
	WorkOrderID   string `json:"work_order_id"`
	PONumber      string `json:"po_number"`
	StyleCode     string `json:"style_code"`
	LineID        string `json:"line_id"`
	SupervisorID  string `json:"supervisor_id"`
	OrderedQty    int    `json:"ordered_qty"`
	ColorJSON     string `json:"color_json"`
	SizeJSON      string `json:"size_json"`
	Status        string `json:"status"`
	PackagingJSON string `json:"packaging_json,omitempty"`
	ShippingJSON  string `json:"shipping_json,omitempty"`
}

func GenerateTemplateCSV() string {
	return `# ==============================================================================
# AGIPAREL OS - EASY FACTORY SETUP WORKBOOK
# ==============================================================================
RECORD_TYPE,COL_1,COL_2,COL_3,COL_4,COL_5,COL_6,COL_7,COL_8,COL_9,COL_10,COL_11
FACTORY_PROFILE,TENANT_DEFAULT,Noble Garments,OKHLA Unit 002/02,8,20
LINE_ROSTER,Line 14,Line 14 - Denim Trouser Assembly,14,600,0.40
LINE_ROSTER,Line 02,Line 02 - Knit Polo Line,12,500,0.40
MACHINE_ROSTER,M-01,Line 14,SUPERVISOR_DESK,Supervisor Terminal,TERMINAL,150,ACTIVE
MACHINE_ROSTER,M-02,Line 14,ST-02,Pegasus 4-Thread Overlock,4-OL_SERGING,120,ACTIVE
MACHINE_ROSTER,M-03,Line 14,ST-03,Kansai Special Flatlock,KANSAI_FLATLOCK,110,ACTIVE
MACHINE_ROSTER,M-04,Line 14,ST-04,Juki DDL-9000C Lockstitch,SNLS_LOCKSTITCH,100,ACTIVE
MACHINE_ROSTER,M-05,Line 14,MAINT_BENCH,Maintenance Service Station,WORKBENCH,60,ACTIVE
WORKER_ROSTER,W-01,EMP_SUPV_01,Ram Kumar,LINE_SUPERVISOR,A,180.00,PRESENT,Line 14,SUPERVISOR_DESK
WORKER_ROSTER,W-02,EMP_A812,Sita Devi,4-OL_SERGING,A,125.00,PRESENT,Line 14,ST-02
WORKER_ROSTER,W-03,EMP_A813,Sunita Sharma,KANSAI_HEMMING,B,115.00,ABSENT,Line 14,ST-03
WORKER_ROSTER,W-04,EMP_A814,Mohan Lal,SNLS_BACK_RISE,A,130.00,PRESENT,Line 14,ST-04
WORKER_ROSTER,W-05,EMP_MECH_01,Ramesh Chand,LINE_MECHANIC,A,160.00,PRESENT,Line 14,MAINT_BENCH
STYLE_MASTER,STYLE_KK524867,KK524867,WOVEN_TROUSER,REGULAR_FIT,0.60,DENIM-STRETCH-12OZ
STYLE_PARTS,KK524867,101,Left Pocket,1,POCKETING-TWILL-WHITE
STYLE_PARTS,KK524867,102,Front Panel,2,DENIM-STRETCH-12OZ
STYLE_PARTS,KK524867,103,Back Panel,3,DENIM-STRETCH-12OZ
STYLE_PARTS,KK524867,104,Waistband,4,DENIM-STRETCH-12OZ
# STYLE_BOM,KK524867,DENIM-STRETCH-12OZ,FABRIC,1.85,M,MARKER-APPROVAL-REFERENCE
OPERATION_DAG,KK524867,OP-101,Attach Left Pocket to Front Panel,0.50,SNLS_LOCKSTITCH,101,102
OPERATION_DAG,KK524867,OP-102,Back Rise & Panel Join,0.60,4-OL_SERGING,103,102
OPERATION_DAG,KK524867,OP-103,Waistband Attach,0.65,KANSAI_FLATLOCK,104,102
STORE_ROLLS,ROLL-007,DENIM-STRETCH-12OZ,59.0,148.0,A,BIN-A01,RESERVED,MK6574J9,1
STORE_ROLLS,ROLL-008,POCKETING-TWILL-WHITE,120.0,150.0,W1,BIN-B02,AVAILABLE,,0
WORK_ORDERS,WO-MK6574J9-01,MK6574J9,KK524867,Line 14,W-01,600,"{""BLK"":150,""BLU"":250,""GRN"":200}","{""29/30"":600}",IN_PROGRESS`
}

func ParseWorkbookCSV(r io.Reader) (*FactoryWorkbook, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	wb := &FactoryWorkbook{
		TenantID:       "",
		ShiftStartHour: 0,
		BundleSize:     0,
	}
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("ERR_CSV_PARSE: %w", err)
	}
	positiveInteger := func(raw, field string, line int) (int, error) {
		n, e := strconv.Atoi(strings.TrimSpace(raw))
		if e != nil || n <= 0 {
			return 0, fmt.Errorf("ERR_INVALID_FIELD row %d %s: expected positive integer; received %q", line, field, raw)
		}
		return n, nil
	}
	positiveDecimal := func(raw, field string, line int) (float64, error) {
		v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 1e12 {
			return 0, fmt.Errorf("ERR_INVALID_FIELD row %d %s: expected positive finite number; received %q", line, field, raw)
		}
		return v, nil
	}
	nonnegativeDecimal := func(raw, field string, line int) (float64, error) {
		v, e := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if e != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 1e12 {
			return 0, fmt.Errorf("ERR_INVALID_FIELD row %d %s: expected nonnegative finite number; received %q", line, field, raw)
		}
		return v, nil
	}

	for lineNum, row := range records {
		if len(row) == 0 {
			continue
		}
		recType := strings.TrimSpace(row[0])
		if recType == "" || strings.HasPrefix(recType, "#") || strings.EqualFold(recType, "RECORD_TYPE") {
			continue
		}
		getCol := func(idx int) string {
			if idx < len(row) {
				return strings.TrimSpace(row[idx])
			}
			return ""
		}

		switch strings.ToUpper(recType) {
		case "FACTORY_PROFILE":
			if getCol(1) != "" {
				wb.TenantID = getCol(1)
			}
			wb.FactoryName = getCol(2)
			wb.UnitCode = getCol(3)
			if getCol(4) != "" {
				sh, err := strconv.Atoi(getCol(4))
				if err != nil || sh < 0 || sh > 23 {
					return nil, fmt.Errorf("ERR_INVALID_FIELD row %d shift_start_hour", lineNum+1)
				}
				wb.ShiftStartHour = sh
			}
			if getCol(5) != "" {
				bs, err := positiveInteger(getCol(5), "bundle_size", lineNum+1)
				if err != nil {
					return nil, err
				}
				wb.BundleSize = bs
			}
		case "LINE_ROSTER":
			stations, err := positiveInteger(getCol(3), "stations_count", lineNum+1)
			if err != nil {
				return nil, err
			}
			capDaily, err := positiveInteger(getCol(4), "target_daily_capacity", lineNum+1)
			if err != nil {
				return nil, err
			}
			dwell, err := positiveDecimal(getCol(5), "dwell_floor_pct", lineNum+1)
			if err != nil {
				return nil, err
			}
			if dwell > 1 {
				return nil, fmt.Errorf("ERR_INVALID_FIELD row %d dwell_floor_pct must not exceed 1", lineNum+1)
			}
			wb.Lines = append(wb.Lines, LineRow{
				LineID:              getCol(1),
				DisplayName:         getCol(2),
				StationsCount:       stations,
				TargetDailyCapacity: capDaily,
				DwellFloorPct:       dwell,
			})
		case "MACHINE_ROSTER":
			capUPH, err := positiveInteger(getCol(6), "capacity_uph", lineNum+1)
			if err != nil {
				return nil, err
			}
			status := strings.ToUpper(getCol(7))
			if status == "" {
				return nil, fmt.Errorf("ERR_INVALID_FIELD row %d machine_status cannot be inferred as ACTIVE", lineNum+1)
			}
			wb.Machines = append(wb.Machines, MachineRow{
				MachineID:      getCol(1),
				LineID:         getCol(2),
				StationID:      getCol(3),
				Name:           getCol(4),
				CanonicalClass: getCol(5),
				CapacityUPH:    capUPH,
				Status:         status,
			})
		case "WORKER_ROSTER":
			rate, err := nonnegativeDecimal(getCol(6), "hourly_rate_inr", lineNum+1)
			if err != nil {
				return nil, err
			}
			grade := strings.ToUpper(getCol(5))
			attStatus := strings.ToUpper(getCol(7))
			if grade == "" || attStatus == "" {
				return nil, fmt.Errorf("ERR_INVALID_FIELD row %d worker grade and attendance must be explicit", lineNum+1)
			}
			wb.Workers = append(wb.Workers, WorkerRow{
				WorkerID:          getCol(1),
				EmployeeCode:      getCol(2),
				Name:              getCol(3),
				PrimarySkill:      getCol(4),
				SkillGrade:        grade,
				HourlyRateINR:     rate,
				AttendanceStatus:  attStatus,
				AssignedLineID:    getCol(8),
				AssignedStationID: getCol(9),
				AttendanceDate:    getCol(10),
			})
		case "STYLE_MASTER":
			sam, err := positiveDecimal(getCol(5), "target_sam_min", lineNum+1)
			if err != nil {
				return nil, err
			}
			wb.Styles = append(wb.Styles, StyleRow{
				StyleID:      getCol(1),
				StyleCode:    getCol(2),
				Category:     getCol(3),
				FitProfile:   getCol(4),
				TargetSAMMin: sam,
				FabricSKU:    getCol(6),
			})
		case "STYLE_PARTS":
			order, err := positiveInteger(getCol(4), "sub_bundle_order", lineNum+1)
			if err != nil {
				return nil, err
			}
			wb.StyleParts = append(wb.StyleParts, StylePartRow{
				StyleCode:      getCol(1),
				PartCode:       getCol(2),
				PartName:       getCol(3),
				SubBundleOrder: order,
				MaterialSKU:    getCol(5),
			})
		case "STYLE_BOM":
			qty, err := positiveDecimal(getCol(4), "qty_per_garment", lineNum+1)
			if err != nil {
				return nil, err
			}
			category, unit := strings.ToUpper(getCol(3)), strings.ToUpper(getCol(5))
			if getCol(1) == "" || getCol(2) == "" || (category != "FABRIC" && category != "TRIM") || unit == "" {
				return nil, fmt.Errorf("ERR_INVALID_FIELD row %d: style, material, category and UOM required", lineNum+1)
			}
			wb.BOMRows = append(wb.BOMRows, BOMRow{
				StyleCode: getCol(1), MaterialSKU: getCol(2), Category: category,
				QtyPerGarment: qty, UOM: unit, SourceRef: getCol(6),
			})
		case "OPERATION_DAG":
			sam, err := positiveDecimal(getCol(4), "sam_minutes", lineNum+1)
			if err != nil {
				return nil, err
			}
			wb.OperationsDAG = append(wb.OperationsDAG, OperationDAGRow{
				StyleCode:      getCol(1),
				OperationID:    getCol(2),
				OperationName:  getCol(3),
				SAMMinutes:     sam,
				MachineType:    getCol(5),
				SourcePartCode: getCol(6),
				TargetPartCode: getCol(7),
			})
		case "STORE_ROLLS":
			lengthM, err := positiveDecimal(getCol(3), "length_m", lineNum+1)
			if err != nil {
				return nil, err
			}
			widthCM, err := positiveDecimal(getCol(4), "width_cm", lineNum+1)
			if err != nil {
				return nil, err
			}
			cutNo := 0
			if getCol(9) != "" {
				cutNo, err = strconv.Atoi(getCol(9))
				if err != nil || cutNo < 0 {
					return nil, fmt.Errorf("ERR_INVALID_FIELD row %d reserved_cut_no", lineNum+1)
				}
			}
			status := strings.ToUpper(getCol(7))
			if status == "" {
				return nil, fmt.Errorf("ERR_INVALID_FIELD row %d roll allocation status cannot be inferred as AVAILABLE", lineNum+1)
			}
			wb.StoreRolls = append(wb.StoreRolls, StoreRollRow{
				RollID:        getCol(1),
				FactorySKU:    getCol(2),
				LengthM:       lengthM,
				WidthCM:       widthCM,
				ShadeBand:     getCol(5),
				BinLocation:   getCol(6),
				Status:        status,
				ReservedOrder: getCol(8),
				ReservedCutNo: cutNo,
			})
		case "WORK_ORDERS":
			qty, err := positiveInteger(getCol(6), "ordered_qty", lineNum+1)
			if err != nil {
				return nil, err
			}
			colorJSON := getCol(7)
			sizeJSON := getCol(8)
			if !json.Valid([]byte(colorJSON)) || !json.Valid([]byte(sizeJSON)) {
				return nil, fmt.Errorf("ERR_INVALID_FIELD row %d missing/invalid color or size matrix; cannot invent DEFAULT", lineNum+1)
			}
			status := strings.ToUpper(getCol(9))
			if status == "" {
				status = "PROPOSED"
			}
			pkgJSON := getCol(10)
			if pkgJSON != "" && !json.Valid([]byte(pkgJSON)) {
				return nil, fmt.Errorf("ERR_INVALID_FIELD row %d invalid packaging_json", lineNum+1)
			}
			shipJSON := getCol(11)
			if shipJSON != "" && !json.Valid([]byte(shipJSON)) {
				return nil, fmt.Errorf("ERR_INVALID_FIELD row %d invalid shipping_json", lineNum+1)
			}
			wb.WorkOrders = append(wb.WorkOrders, WorkOrderRow{
				WorkOrderID:   getCol(1),
				PONumber:      getCol(2),
				StyleCode:     getCol(3),
				LineID:        getCol(4),
				SupervisorID:  getCol(5),
				OrderedQty:    qty,
				ColorJSON:     colorJSON,
				SizeJSON:      sizeJSON,
				Status:        status,
				PackagingJSON: pkgJSON,
				ShippingJSON:  shipJSON,
			})
		default:
			return nil, fmt.Errorf("ERR_UNKNOWN_RECORD_TYPE: unknown record type '%s' at row %d", recType, lineNum+1)
		}
	}
	if wb.FactoryName == "" {
		return nil, errors.New("ERR_MISSING_FACTORY_PROFILE: Workbook must include a FACTORY_PROFILE row")
	}
	return wb, nil
}

// ApplyToDatabase is intentionally retired. No source CSV, AI parse or legacy
// onboarding workbook may populate factory operational tables directly.
// Use reviewed, versioned domain publication APIs introduced in Gates B2-B4.
func ApplyToDatabase(ctx context.Context, db *sql.DB, wb *FactoryWorkbook) error {
	return errors.New("ERR_LEGACY_IMPORT_RETIRED: direct workbook publication is disabled; use /documents and /factory-setup")
}
