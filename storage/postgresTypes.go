package storage

type PostgresGuild struct {
	GuildID    uint64 `db:"guild_id"`
	GuildName  string `db:"guild_name"`
	Premium    int16  `db:"premium"`
	TxTimeUnix *int32 `db:"tx_time_unix"`
}

type PostgresGame struct {
	GameID      int64  `db:"game_id"`
	GuildID     uint64 `db:"guild_id"`
	ConnectCode string `db:"connect_code"`
	StartTime   int32  `db:"start_time"`
	WinType     int16  `db:"win_type"`
	EndTime     int32  `db:"end_time"`
}

type PostgresUser struct {
	UserID uint64 `db:"user_id"`
	Opt    bool   `db:"opt"`
}

type PostgresUserGame struct {
	UserID      uint64 `db:"user_id"`
	GuildID     uint64 `db:"guild_id"`
	GameID      int64  `db:"game_id"`
	PlayerName  string `db:"player_name"`
	PlayerColor int16  `db:"player_color"`
	PlayerRole  int16  `db:"player_role"`
	PlayerWon   bool   `db:"player_won"`
}

type PostgresGameEvent struct {
	EventID   uint64  `db:"event_id"`
	UserID    *uint64 `db:"user_id"`
	GameID    int64   `db:"game_id"`
	EventTime int32   `db:"event_time"`
	EventType int16   `db:"event_type"`
	Payload   string  `db:"payload"`
}

type PostgresOtherPlayerRanking struct {
	UserID  uint64  `db:"user_id"`
	Count   int64   `db:"count"`
	Percent float64 `db:"percent"`
}

type PostgresPlayerRanking struct {
	UserID   uint64  `db:"user_id"`
	WinCount int64   `db:"win"`
	Count    int64   `db:"total"`
	WinRate  float64 `db:"win_rate"`
}
