package utils

import (
	"log"
	"os"
)

type Logger struct {
	prefix string
	logger *log.Logger
}

func Log(prefix string) *Logger {
	return &Logger{
		prefix: prefix,
		logger: log.New(os.Stdout, "", log.LstdFlags),
	}
}

func (l *Logger) Log(v ...interface{}) {
	prefix := "[" + l.prefix + "] "
	l.logger.Println(append([]interface{}{prefix}, v...)...)
}

func (l *Logger) Logf(format string, v ...interface{}) {
	prefix := "[" + l.prefix + "] "
	l.logger.Printf(prefix+format+"\n", v...)
}

var (
	Backend    = Log("BACKEND")
	Discord    = Log("DISCORD")
	XMPP       = Log("XMPP")
	Matchmaker = Log("MATCHMAKER")
	MongoDB    = Log("MONGODB")
	Warning    = Log("WARNING")
	Error      = Log("ERROR")
	OAuth2     = Log("OAUTH2")
)
