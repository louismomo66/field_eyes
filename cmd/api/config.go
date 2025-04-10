package main

import (
	"database/sql"
	"field_eyes/data"
	"field_eyes/pkg/email"
	"log"
	"sync"
)

type Config struct {
	DB            *sql.DB
	InfoLog       *log.Logger
	ErrorLog      *log.Logger
	Wait          *sync.WaitGroup
	Models        data.Models
	Mailer        email.Mailer
	Redis         *RedisClient
	MQTT          *MQTTClient
	ErrorChan     chan error
	ErrorChanDone chan bool
}
