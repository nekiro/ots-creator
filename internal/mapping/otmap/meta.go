package otmap

type House struct {
	ID        uint32   `json:"id"`
	Name      string   `json:"name"`
	TownID    uint32   `json:"townId"`
	Rent      int32    `json:"rent"`
	Guildhall bool     `json:"guildhall"`
	Exit      Position `json:"exit"`
	// ClientID and Beds are attributes of newer servers.
	ClientID uint32 `json:"clientId,omitempty"`
	Beds     int32  `json:"beds,omitempty"`
}

func (h *House) Clone() *House {
	clone := *h
	return &clone
}

type Town struct {
	ID     uint32   `json:"id"`
	Name   string   `json:"name"`
	Temple Position `json:"temple"`
}

type Waypoint struct {
	Name string   `json:"name"`
	Pos  Position `json:"pos"`
}

type Zone struct {
	ID   uint16 `json:"id"`
	Name string `json:"name"`
}
