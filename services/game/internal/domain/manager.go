package game

// Manager will own one serialized command loop per active game. It is kept in
// the domain package so transports never mutate Game directly.
type Manager interface {
	Dispatch(Command) error
}

type Command struct {
	GameID  string
	ActorID string
	Type    string
	Payload any
}
