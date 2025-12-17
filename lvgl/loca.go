package lvgl

type LocaTable struct {
	Size       uint32  //4	Record size (for quick skip)
	Label      [4]byte //4	"loca"
	EntryCount uint32  //4	Entries count (4 to simplify slign)
	// 然后是 offsets：[]uint16 或 []uint32
}

func NewLocaTable() *LocaTable {
	return &LocaTable{
		Size:       12,
		Label:      [4]byte{'l', 'o', 'c', 'a'},
		EntryCount: 0,
	}
}
