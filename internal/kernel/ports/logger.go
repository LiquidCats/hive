package ports

type Logger[F any] interface {
	Debug(msg string, args ...F)
	Info(msg string, args ...F)
	Warn(msg string, args ...F)
	Error(msg string, args ...F)
}
