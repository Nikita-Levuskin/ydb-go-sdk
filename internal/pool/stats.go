package pool

type Stats struct {
	Limit            int
	Index            int
	Idle             int
	CreateInProgress int
}
