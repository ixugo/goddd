package logger

type Option func(*Slog)

func WithEvents(events Events) Option {
	return func(s *Slog) {
		s.events = events
	}
}
