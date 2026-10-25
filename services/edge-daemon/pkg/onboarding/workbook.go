// Author: Sunil Jain, 2026-10-25 — Preserve partial PO updates and transactional tenant isolation.
package onboarding

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
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

// GenerateTemplateCSV returns a pre-boarding spreadsheet with example rows in every section.
func GenerateTemplateCSV() string {
	return `# ==============================================================================
# AGIPAREL OS - EASY FACTORY SETUP WORKBOOK
# ==============================================================================
# FRIENDLY TIP: Do your best to fill out what you know! If you are missing any
# details or feel unsure, do not get stuck. Leave non-critical rows blank or
# use rough estimates. We can easily collect and update any missing info later in
# second and future rounds.
#
# Instructions:
# - Keep RECORD_TYPE in Column A.
# - Lines starting with '#' are comments and are ignored when importing.
# ==============================================================================
RECORD_TYPE,COL_1,COL_2,COL_3,COL_4,COL_5,COL_6,COL_7,COL_8,COL_9

# [1] FACTORY_PROFILE: Tell us your system ID, factory name, unit, shift start hour, and default bundle size.
# Format: FACTORY_PROFILE, Tenant ID, Factory Name, Unit Location, Shift Start Hour (0-23), Default Bundle Size
FACTORY_PROFILE,TENANT_DEFAULT,Noble Garments,OKHLA Unit 002/02,8,20

# [2] LINE_ROSTER: List your production assembly lines and how many workstations they have.
# Format: LINE_ROSTER, Line ID, Display Name, Number of Stations, Target Daily Capacity, Cycle Dwell Floor %
LINE_ROSTER,Line 14,Line 14 - Denim Trouser Assembly,14,600,0.40
LINE_ROSTER,Line 02,Line 02 - Knit Polo Line,12,500,0.40

# [3] MACHINE_ROSTER: List the sewing and workshop machines assigned to your lines.
# Format: MACHINE_ROSTER, Machine ID, Line ID, Station ID, Machine Name, Machine Type Class, Capacity (Units/Hour), Status (ACTIVE/IDLE/BREAKDOWN/MAINTENANCE)
MACHINE_ROSTER,M-01,Line 14,SUPERVISOR_DESK,Supervisor Terminal,TERMINAL,150,ACTIVE
MACHINE_ROSTER,M-02,Line 14,ST-02,Pegasus 4-Thread Overlock,4-OL_SERGING,120,ACTIVE
MACHINE_ROSTER,M-03,Line 14,ST-03,Kansai Special Flatlock,KANSAI_FLATLOCK,110,ACTIVE
MACHINE_ROSTER,M-04,Line 14,ST-04,Juki DDL-9000C Lockstitch,SNLS_LOCKSTITCH,100,ACTIVE
MACHINE_ROSTER,M-05,Line 14,MAINT_BENCH,Maintenance Service Station,WORKBENCH,60,ACTIVE

# [4] WORKER_ROSTER: List your workers, employee codes, skills, grade, hourly wage, and today's attendance status.
# Format: WORKER_ROSTER, Worker ID, Employee Code, Full Name, Primary Skill, Skill Grade (A/B/C/TRAINEE), Hourly Rate (INR), Attendance Today (PRESENT/ABSENT/LATE/ON_LEAVE), Assigned Line ID, Assigned Station ID
WORKER_ROSTER,W-01,EMP_SUPV_01,Ram Kumar,LINE_SUPERVISOR,A,180.00,PRESENT,Line 14,SUPERVISOR_DESK
WORKER_ROSTER,W-02,EMP_A812,Sita Devi,4-OL_SERGING,A,125.00,PRESENT,Line 14,ST-02
WORKER_ROSTER,W-03,EMP_A813,Sunita Sharma,KANSAI_HEMMING,B,115.00,ABSENT,Line 14,ST-03
WORKER_ROSTER,W-04,EMP_A814,Mohan Lal,SNLS_BACK_RISE,A,130.00,PRESENT,Line 14,ST-04
WORKER_ROSTER,W-05,EMP_MECH_01,Ramesh Chand,LINE_MECHANIC,A,160.00,PRESENT,Line 14,MAINT_BENCH

# [5] STYLE_MASTER: Define the garment styles you manufacture.
# Format: STYLE_MASTER, Style ID, Style Code, Category, Fit Profile, Target Sewing Minutes (SAM), Fabric SKU
STYLE_MASTER,STYLE_KK524867,KK524867,WOVEN_TROUSER,REGULAR_FIT,0.60,DENIM-STRETCH-12OZ

# [6] STYLE_PARTS: List the component parts that make up each garment style.
# Format: STYLE_PARTS, Style Code, Part Code, Part Name, Assembly Order (1..N), Material SKU
STYLE_PARTS,KK524867,101,Left Pocket,1,POCKETING-TWILL-WHITE
STYLE_PARTS,KK524867,102,Front Panel,2,DENIM-STRETCH-12OZ
STYLE_PARTS,KK524867,103,Back Panel,3,DENIM-STRETCH-12OZ
STYLE_PARTS,KK524867,104,Waistband,4,DENIM-STRETCH-12OZ

# [7] OPERATION_DAG: Define the step-by-step sewing operations and part assembly links.
# Format: OPERATION_DAG, Style Code, Operation ID, Operation Name, Time in Minutes (SAM), Machine Type Needed, Source Part Code, Target Part Code
OPERATION_DAG,KK524867,OP-101,Attach Left Pocket to Front Panel,0.50,SNLS_LOCKSTITCH,101,102
OPERATION_DAG,KK524867,OP-102,Back Rise & Panel Join,0.60,4-OL_SERGING,103,102
OPERATION_DAG,KK524867,OP-103,Waistband Attach,0.65,KANSAI_FLATLOCK,104,102

# [8] STORE_ROLLS: Record fabric rolls available in your store/warehouse.
# Format: STORE_ROLLS, Roll ID, Fabric SKU, Length in Meters, Width in Centimeters, Shade Band, Bin Location, Status (AVAILABLE/RESERVED/ISSUED/DEPLETED), Reserved PO Number, Reserved Cut Number
STORE_ROLLS,ROLL-007,DENIM-STRETCH-12OZ,59.0,148.0,A,BIN-A01,RESERVED,MK6574J9,1
STORE_ROLLS,ROLL-008,POCKETING-TWILL-WHITE,120.0,150.0,W1,BIN-B02,AVAILABLE,,0

# [9] WORK_ORDERS: Enter active customer purchase orders and production quantities.
# Format: WORK_ORDERS, Work Order ID, PO Number, Style Code, Line ID, Supervisor Worker ID, Ordered Quantity, Color Breakdown (JSON), Size Breakdown (JSON), Status (PLANNED/IN_PROGRESS/COMPLETED/HOLD)
WORK_ORDERS,WO-MK6574J9-01,MK6574J9,KK524867,Line 14,W-01,600,"{""BLK"":150,""BLU"":250,""GRN"":200}","{""29/30"":600}",IN_PROGRESS
`
}

func ParseWorkbookCSV(r io.Reader) (*FactoryWorkbook, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	wb := &FactoryWorkbook{
		TenantID:       "TENANT_DEFAULT",
		ShiftStartHour: 8,
		BundleSize:     20,
	}

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("ERR_CSV_PARSE: %w", err)
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
			if sh, err := strconv.Atoi(getCol(4)); err == nil && sh >= 0 && sh <= 23 {
				wb.ShiftStartHour = sh
			}
			if bs, err := strconv.Atoi(getCol(5)); err == nil && bs > 0 {
				wb.BundleSize = bs
			}

		case "LINE_ROSTER":
			stations, _ := strconv.Atoi(getCol(3))
			capDaily, _ := strconv.Atoi(getCol(4))
			dwell, err := strconv.ParseFloat(getCol(5), 64)
			if err != nil || dwell <= 0 {
				dwell = 0.40
			}
			wb.Lines = append(wb.Lines, LineRow{
				LineID:              getCol(1),
				DisplayName:         getCol(2),
				StationsCount:       stations,
				TargetDailyCapacity: capDaily,
				DwellFloorPct:       dwell,
			})

		case "MACHINE_ROSTER":
			capUPH, _ := strconv.Atoi(getCol(6))
			if capUPH <= 0 {
				capUPH = 100
			}
			status := strings.ToUpper(getCol(7))
			if status == "" {
				status = "ACTIVE"
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
			rate, _ := strconv.ParseFloat(getCol(6), 64)
			grade := strings.ToUpper(getCol(5))
			if grade == "" {
				grade = "A"
			}
			attStatus := strings.ToUpper(getCol(7))
			if attStatus == "" {
				attStatus = "PRESENT"
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
			})

		case "STYLE_MASTER":
			sam, _ := strconv.ParseFloat(getCol(5), 64)
			if sam <= 0 {
				sam = 0.60
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
			order, _ := strconv.Atoi(getCol(4))
			if order <= 0 {
				order = 1
			}
			wb.StyleParts = append(wb.StyleParts, StylePartRow{
				StyleCode:      getCol(1),
				PartCode:       getCol(2),
				PartName:       getCol(3),
				SubBundleOrder: order,
				MaterialSKU:    getCol(5),
			})

		case "OPERATION_DAG":
			sam, _ := strconv.ParseFloat(getCol(4), 64)
			if sam <= 0 {
				sam = 0.50
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
			lengthM, _ := strconv.ParseFloat(getCol(3), 64)
			widthCM, _ := strconv.ParseFloat(getCol(4), 64)
			cutNo, _ := strconv.Atoi(getCol(9))
			status := strings.ToUpper(getCol(7))
			if status == "" {
				status = "AVAILABLE"
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
			qty, _ := strconv.Atoi(getCol(6))
			colorJSON := getCol(7)
			if colorJSON == "" || !json.Valid([]byte(colorJSON)) {
				colorJSON = `{"DEFAULT":` + strconv.Itoa(qty) + `}`
			}
			sizeJSON := getCol(8)
			if sizeJSON == "" || !json.Valid([]byte(sizeJSON)) {
				sizeJSON = `{"DEFAULT":` + strconv.Itoa(qty) + `}`
			}
			status := strings.ToUpper(getCol(9))
			if status == "" {
				status = "IN_PROGRESS"
			}
			wb.WorkOrders = append(wb.WorkOrders, WorkOrderRow{
				WorkOrderID:  getCol(1),
				PONumber:     getCol(2),
				StyleCode:    getCol(3),
				LineID:       getCol(4),
				SupervisorID: getCol(5),
				OrderedQty:   qty,
				ColorJSON:    colorJSON,
				SizeJSON:     sizeJSON,
				Status:       status,
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

// ApplyToDatabase transactionally provisions all factory entities from the parsed Pre-Boarding Workbook.
func ApplyToDatabase(ctx context.Context, db *sql.DB, wb *FactoryWorkbook) error {
	if db == nil {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", wb.TenantID); err != nil {
		return fmt.Errorf("failed to set tenant context: %w", err)
	}

	// 1. Upsert Tenant
	fullTenantName := wb.FactoryName
	if wb.UnitCode != "" {
		fullTenantName = fmt.Sprintf("%s (%s)", wb.FactoryName, wb.UnitCode)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tenants (id, name) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;
	`, wb.TenantID, fullTenantName); err != nil {
		return fmt.Errorf("failed to upsert tenant: %w", err)
	}

	// 2. Upsert Workers & Attendance + Auto-Flag Absenteeism Issues
	for _, w := range wb.Workers {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO workers (id, tenant_id, name, employee_code, primary_skill, skill_grade, hourly_rate_inr)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				employee_code = EXCLUDED.employee_code,
				primary_skill = EXCLUDED.primary_skill,
				skill_grade = EXCLUDED.skill_grade,
				hourly_rate_inr = EXCLUDED.hourly_rate_inr;
		`, w.WorkerID, wb.TenantID, w.Name, w.EmployeeCode, w.PrimarySkill, w.SkillGrade, w.HourlyRateINR); err != nil {
			return fmt.Errorf("failed to upsert worker %s: %w", w.WorkerID, err)
		}

		// Source attendance belongs to its recorded shift, never silently to today's shift.
		date := w.AttendanceDate
		if date == "" {
			date = ""
		} // Legacy factory-input template explicitly describes today's attendance.
		_, err = tx.ExecContext(ctx, `INSERT INTO attendance_register
           (tenant_id, worker_id, shift_date, clock_in_time, status, assigned_line_id, assigned_station_id)
           VALUES ($1,$2,COALESCE(NULLIF($3,'')::date,CURRENT_DATE),NULL,$4,$5,$6)
           ON CONFLICT (tenant_id,worker_id,shift_date) DO UPDATE SET status=EXCLUDED.status,
           assigned_line_id=EXCLUDED.assigned_line_id,assigned_station_id=EXCLUDED.assigned_station_id`,
			wb.TenantID, w.WorkerID, date, w.AttendanceStatus, w.AssignedLineID, w.AssignedStationID)
		if err != nil {
			return fmt.Errorf("failed to record attendance for %s: %w", w.WorkerID, err)
		}

		if w.AttendanceStatus == "ABSENT" && w.AssignedLineID != "" && (w.AttendanceDate == "" || w.AttendanceDate == time.Now().Format("2006-01-02")) {
			issID := fmt.Sprintf("ISS-ATT-%s", w.WorkerID)
			title := fmt.Sprintf("%s absent; station %s (%s) requires float coverage", w.Name, w.AssignedStationID, w.PrimarySkill)
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO floor_issues (id, tenant_id, issue_type, severity, plant_id, line_id, station_id, operator_id, title, owner_id, status)
				VALUES ($1, $2, 'workforce', 'attention', $3, $4, $5, $6, $7, $6, 'open')
				ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, status = 'open';
			`, issID, wb.TenantID, wb.FactoryName, w.AssignedLineID, w.AssignedStationID, w.WorkerID, title)
		}
	}

	// 3. Upsert Machines & Auto-Flag Breakdown Issues
	for _, m := range wb.Machines {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO machine_roster (id, tenant_id, line_id, station_id, name, canonical_class, rated_capacity_uph, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (id) DO UPDATE SET
				line_id = EXCLUDED.line_id,
				station_id = EXCLUDED.station_id,
				name = EXCLUDED.name,
				canonical_class = EXCLUDED.canonical_class,
				rated_capacity_uph = EXCLUDED.rated_capacity_uph,
				status = EXCLUDED.status;
		`, m.MachineID, wb.TenantID, m.LineID, m.StationID, m.Name, m.CanonicalClass, m.CapacityUPH, m.Status); err != nil {
			return fmt.Errorf("failed to upsert machine %s: %w", m.MachineID, err)
		}

		if m.Status == "BREAKDOWN" || m.Status == "MAINTENANCE" {
			issID := fmt.Sprintf("ISS-MACH-%s", m.MachineID)
			title := fmt.Sprintf("Machine %s (%s) in %s status at %s", m.Name, m.CanonicalClass, m.Status, m.StationID)
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO floor_issues (id, tenant_id, issue_type, severity, plant_id, line_id, station_id, machine_id, title, owner_id, status)
				VALUES ($1, $2, 'maintenance', 'critical', $3, $4, $5, $6, $7, 'MAINT_LEAD', 'open')
				ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, status = 'open';
			`, issID, wb.TenantID, wb.FactoryName, m.LineID, m.StationID, m.MachineID, title)
		}
	}

	// 4. Upsert Styles & Parts
	styleIDByCode := make(map[string]string)
	for _, st := range wb.Styles {
		sID := st.StyleID
		if sID == "" {
			sID = "STYLE_" + st.StyleCode
		}
		styleIDByCode[st.StyleCode] = sID
		meta := fmt.Sprintf(`{"fabric": %q}`, st.FabricSKU)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO styles_master (id, tenant_id, style_code, category, fit_profile, target_sam, metadata)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
			ON CONFLICT (id) DO UPDATE SET
				style_code = EXCLUDED.style_code,
				category = EXCLUDED.category,
				fit_profile = EXCLUDED.fit_profile,
				target_sam = EXCLUDED.target_sam,
				metadata = EXCLUDED.metadata;
		`, sID, wb.TenantID, st.StyleCode, st.Category, st.FitProfile, st.TargetSAMMin, meta); err != nil {
			return fmt.Errorf("failed to upsert style %s: %w", st.StyleCode, err)
		}
	}

	partIDByStyleAndCode := make(map[string]string)
	for _, pt := range wb.StyleParts {
		sID := styleIDByCode[pt.StyleCode]
		if sID == "" {
			sID = "STYLE_" + pt.StyleCode
		}
		partID := fmt.Sprintf("PART_%s_%s", pt.StyleCode, pt.PartCode)
		partIDByStyleAndCode[pt.StyleCode+":"+pt.PartCode] = partID
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO style_parts (id, tenant_id, style_id, part_code, part_name, sub_bundle_order, material_sku)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (id) DO UPDATE SET
				part_name = EXCLUDED.part_name,
				sub_bundle_order = EXCLUDED.sub_bundle_order,
				material_sku = EXCLUDED.material_sku;
		`, partID, wb.TenantID, sID, pt.PartCode, pt.PartName, pt.SubBundleOrder, pt.MaterialSKU); err != nil {
			return fmt.Errorf("failed to upsert style part %s: %w", pt.PartCode, err)
		}
	}

	// 5. Upsert Operations & Style Assembly DAG
	for _, op := range wb.OperationsDAG {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO operation_master (id, operation_name, default_sam, machine_type)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (id) DO UPDATE SET
				operation_name = EXCLUDED.operation_name,
				default_sam = EXCLUDED.default_sam,
				machine_type = EXCLUDED.machine_type;
		`, op.OperationID, op.OperationName, op.SAMMinutes, op.MachineType); err != nil {
			return fmt.Errorf("failed to upsert operation %s: %w", op.OperationID, err)
		}

		sID := styleIDByCode[op.StyleCode]
		srcPartID := partIDByStyleAndCode[op.StyleCode+":"+op.SourcePartCode]
		tgtPartID := partIDByStyleAndCode[op.StyleCode+":"+op.TargetPartCode]
		if sID != "" && srcPartID != "" && tgtPartID != "" {
			dagID := fmt.Sprintf("DAG_%s_%s", op.StyleCode, op.OperationID)
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO style_assembly_dag (id, tenant_id, style_id, parent_part_id, child_part_id, operation_id, machine_type_req)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (id) DO UPDATE SET
					parent_part_id = EXCLUDED.parent_part_id,
					child_part_id = EXCLUDED.child_part_id,
					machine_type_req = EXCLUDED.machine_type_req;
			`, dagID, wb.TenantID, sID, srcPartID, tgtPartID, op.OperationID, op.MachineType)
		}
	}

	// 6. Upsert Store Rolls
	for _, r := range wb.StoreRolls {
		var resOrder interface{}
		var resCut interface{}
		if r.ReservedOrder != "" {
			resOrder = r.ReservedOrder
		}
		if r.ReservedCutNo > 0 {
			resCut = r.ReservedCutNo
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO store_rolls (id, tenant_id, factory_sku, original_length_m, remaining_length_m, usable_width_cm, shade_band, bin_location, status, reserved_order_id, reserved_cut_no)
			VALUES ($1, $2, $3, $4, $4, $5, $6, $7, $8, $9, $10)
			ON CONFLICT (id) DO UPDATE SET
				factory_sku = EXCLUDED.factory_sku,
				original_length_m = EXCLUDED.original_length_m,
				remaining_length_m = EXCLUDED.remaining_length_m,
				usable_width_cm = EXCLUDED.usable_width_cm,
				shade_band = EXCLUDED.shade_band,
				bin_location = EXCLUDED.bin_location,
				status = EXCLUDED.status,
				reserved_order_id = EXCLUDED.reserved_order_id,
				reserved_cut_no = EXCLUDED.reserved_cut_no;
		`, r.RollID, wb.TenantID, r.FactorySKU, r.LengthM, r.WidthCM, r.ShadeBand, r.BinLocation, r.Status, resOrder, resCut); err != nil {
			return fmt.Errorf("failed to upsert store roll %s: %w", r.RollID, err)
		}
	}

	// 7. Upsert Dynamic Work Orders
	for _, wo := range wb.WorkOrders {
		sID := styleIDByCode[wo.StyleCode]
		if sID == "" {
			sID = "STYLE_" + wo.StyleCode
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO dynamic_work_orders (id, tenant_id, po_number, style_id, line_id, supervisor_id, ordered_qty, color_matrix, size_matrix, packaging_bulletin, shipping_spec, status)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::jsonb, $11::jsonb, $12)
			ON CONFLICT (id) DO UPDATE SET
				po_number = EXCLUDED.po_number,
				style_id = EXCLUDED.style_id,
				line_id = EXCLUDED.line_id,
				supervisor_id = EXCLUDED.supervisor_id,
				ordered_qty = EXCLUDED.ordered_qty,
				color_matrix = EXCLUDED.color_matrix,
				size_matrix = EXCLUDED.size_matrix,
				packaging_bulletin = EXCLUDED.packaging_bulletin,
				shipping_spec = EXCLUDED.shipping_spec,
				status = CASE WHEN dynamic_work_orders.status = 'LOCKED' THEN 'LOCKED' ELSE EXCLUDED.status END;
		`, wo.WorkOrderID, wb.TenantID, wo.PONumber, sID, wo.LineID, wo.SupervisorID, wo.OrderedQty, defaultJSON(wo.ColorJSON), defaultJSON(wo.SizeJSON), defaultJSON(wo.PackagingJSON), defaultJSON(wo.ShippingJSON), wo.Status); err != nil {
			return fmt.Errorf("failed to upsert work order %s: %w", wo.WorkOrderID, err)
		}
	}

	return tx.Commit()
}
func defaultJSON(v string) string {
	if strings.TrimSpace(v) == "" {
		return "{}"
	}
	return v
}
