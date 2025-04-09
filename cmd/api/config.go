package main

import (
	"field_eyes/data"
	"field_eyes/pkg/email"
	"log"
	"sync"

	"gorm.io/gorm"
)

type Config struct {
	DB            *gorm.DB
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
