package topology

import (
	"sync"
	"time"
)

type DeviceType string

const (
	DevicePhone      DeviceType = "PHONE_HMI"
	DeviceTablet     DeviceType = "TABLET_KIOSK"
	DeviceSled       DeviceType = "RFID_SLED"
	DeviceScanner    DeviceType = "BT_RING_SCANNER"
	DeviceScale      DeviceType = "DIGITAL_SCALE"
	DeviceEdgeServer DeviceType = "EDGE_SERVER"
)

type EdgeDevice struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Type       DeviceType `json:"type"`
	IP         string     `json:"ip"`
	Mac        string     `json:"mac"`
	Zone       string     `json:"zone"`
	StationID  string     `json:"station_id,omitempty"`
	PosX       int        `json:"pos_x"` // Coordinates (0-1000 grid)
	PosY       int        `json:"pos_y"`
	Status     string     `json:"status"` // ONLINE, WARNING, OFFLINE
	LastSeen   int64      `json:"last_seen"`
	PingMs     int        `json:"ping_ms"`
	ActiveUser string     `json:"active_user,omitempty"`
}

type SpatialTwin struct {
	sync.RWMutex
	devices map[string]*EdgeDevice
}

func NewSpatialTwin() *SpatialTwin {
	t := &SpatialTwin{
		devices: make(map[string]*EdgeDevice),
	}
	t.seedGroundTruthDevices()
	return t
}

func (t *SpatialTwin) seedGroundTruthDevices() {
	now := time.Now().UTC().UnixMilli()
	defaults := []EdgeDevice{
		{ID: "ED-01", Name: "Stores Handheld Sled", Type: DeviceSled, IP: "192.168.10.21", Mac: "48:27:EA:01:A1:01", Zone: "STORES", PosX: 110, PosY: 180, Status: "ONLINE", LastSeen: now, PingMs: 4, ActiveUser: "STORE_USER_01"},
		{ID: "ED-02", Name: "Cutting Bundler Tablet", Type: DeviceTablet, IP: "192.168.10.22", Mac: "48:27:EA:01:A1:02", Zone: "CUTTING", PosX: 300, PosY: 180, Status: "ONLINE", LastSeen: now, PingMs: 3, ActiveUser: "CUT_HELPER_01"},
		{ID: "ED-03", Name: "Sewing ST-01 HMI", Type: DevicePhone, IP: "192.168.10.31", Mac: "48:27:EA:01:B1:01", Zone: "LINE_14", StationID: "LINE14_ST01", PosX: 520, PosY: 100, Status: "ONLINE", LastSeen: now, PingMs: 2, ActiveUser: "OP_A811"},
		{ID: "ED-04", Name: "Sewing ST-02 HMI", Type: DevicePhone, IP: "192.168.10.32", Mac: "48:27:EA:01:B1:02", Zone: "LINE_14", StationID: "LINE14_ST02", PosX: 520, PosY: 170, Status: "ONLINE", LastSeen: now, PingMs: 3, ActiveUser: "OP_A812"},
		{ID: "ED-05", Name: "Sewing ST-03 HMI", Type: DevicePhone, IP: "192.168.10.33", Mac: "48:27:EA:01:B1:03", Zone: "LINE_14", StationID: "LINE14_ST03", PosX: 520, PosY: 240, Status: "ONLINE", LastSeen: now, PingMs: 2, ActiveUser: "OP_A813"},
		{ID: "ED-06", Name: "Sewing ST-04 HMI", Type: DevicePhone, IP: "192.168.10.34", Mac: "48:27:EA:01:B1:04", Zone: "LINE_14", StationID: "LINE14_ST04", PosX: 620, PosY: 100, Status: "ONLINE", LastSeen: now, PingMs: 2, ActiveUser: "OP_A814"},
		{ID: "ED-07", Name: "Sewing ST-05 HMI", Type: DevicePhone, IP: "192.168.10.35", Mac: "48:27:EA:01:B1:05", Zone: "LINE_14", StationID: "LINE14_ST05", PosX: 620, PosY: 170, Status: "ONLINE", LastSeen: now, PingMs: 4, ActiveUser: "OP_A815"},
		{ID: "ED-08", Name: "Sewing ST-06 HMI", Type: DevicePhone, IP: "192.168.10.36", Mac: "48:27:EA:01:B1:06", Zone: "LINE_14", StationID: "LINE14_ST06", PosX: 620, PosY: 240, Status: "ONLINE", LastSeen: now, PingMs: 3, ActiveUser: "OP_A816"},
		{ID: "ED-09", Name: "Finishing Final QC Tablet", Type: DeviceTablet, IP: "192.168.10.41", Mac: "48:27:EA:01:C1:01", Zone: "FINISHING", PosX: 790, PosY: 130, Status: "ONLINE", LastSeen: now, PingMs: 5, ActiveUser: "QC_LEAD_01"},
		{ID: "ED-10", Name: "QC Bluetooth Ring Scanner", Type: DeviceScanner, IP: "192.168.10.42", Mac: "48:27:EA:01:C1:02", Zone: "FINISHING", PosX: 790, PosY: 210, Status: "ONLINE", LastSeen: now, PingMs: 8, ActiveUser: "QC_LEAD_01"},
		{ID: "ED-11", Name: "Ratio Packing Station Tablet", Type: DeviceTablet, IP: "192.168.10.51", Mac: "48:27:EA:01:D1:01", Zone: "PACKING", PosX: 920, PosY: 130, Status: "ONLINE", LastSeen: now, PingMs: 3, ActiveUser: "PACKER_01"},
		{ID: "ED-12", Name: "Benchtop Scale 30kg", Type: DeviceScale, IP: "192.168.10.52", Mac: "48:27:EA:01:D1:02", Zone: "PACKING", PosX: 920, PosY: 210, Status: "ONLINE", LastSeen: now, PingMs: 6, ActiveUser: "PACKER_01"},
		{ID: "ED-13", Name: "plant-edge-os Industrial Gateway", Type: DeviceEdgeServer, IP: "192.168.10.10", Mac: "48:27:EA:01:00:01", Zone: "SERVER_RACK", PosX: 570, PosY: 340, Status: "ONLINE", LastSeen: now, PingMs: 0, ActiveUser: "SYSTEM_ROOT"},
		{ID: "ED-14", Name: "Line Supervisor Mobile Tablet", Type: DeviceTablet, IP: "192.168.10.99", Mac: "48:27:EA:01:FF:01", Zone: "SUPERVISOR", PosX: 430, PosY: 340, Status: "ONLINE", LastSeen: now, PingMs: 5, ActiveUser: "SUPV_RAM_KUMAR"},
	}

	for _, d := range defaults {
		dev := d
		t.devices[d.ID] = &dev
	}
}

func (t *SpatialTwin) GetDevices() []EdgeDevice {
	t.RLock()
	defer t.RUnlock()
	res := make([]EdgeDevice, 0, len(t.devices))
	for _, d := range t.devices {
		res = append(res, *d)
	}
	return res
}

func (t *SpatialTwin) UpdateHeartbeat(deviceID string, pingMs int) {
	t.Lock()
	defer t.Unlock()
	if dev, ok := t.devices[deviceID]; ok {
		dev.LastSeen = time.Now().UTC().UnixMilli()
		dev.PingMs = pingMs
		dev.Status = "ONLINE"
	}
}
