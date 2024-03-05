package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	_ "os"
	"sync"
	"time"
	// "syscall"

	pb "gprs/proto/proto"

	"golang.org/x/crypto/ssh/terminal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Entry struct {
	Session_id string
	Frequency  float64
	Timestamp  time.Time
}

var entryPool = sync.Pool{
	New: func() interface{} {
		return &pb.Entry{}
	},
}

func getEntry() *pb.Entry {
	return entryPool.Get().(*pb.Entry)
}

func putEntry(entry *pb.Entry) {
	entry.Reset()
	entryPool.Put(entry)
}

type Stats struct {
	count int
	mean  float64
	std   float64
}

func (p *Stats) Approximate(frequency float64) {
	p.count++
	delta := frequency - p.mean
	p.mean += delta / float64(p.count)
	p.std = delta * (frequency - p.mean)
}

func main() {
	k := flag.Float64("k", 7, "")
	flag.Parse()

	fmt.Println("Enter login for the database:")
	login, err := terminal.ReadPassword(0)
	fmt.Println("Enter password for the database:")
	password, err := terminal.ReadPassword(0)

	conn, err := grpc.Dial("localhost:8080", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Error at the dail: %v", err)
	}
	defer conn.Close()

	c := pb.NewTransmitterClient(conn)
	stream, err := c.GetEntry(context.Background(), &emptypb.Empty{})
	if err != nil {
		log.Fatalf("Could not get entry: %v", err)
	}

	// For logging in 01 Task:
	// logFile, err := os.Create("log.txt")
	// if err != nil {
	// 	log.Fatalf("Could not create logfile: %v", err)
	// }
	// defer logFile.Close()
	// log.SetOutput(logFile)

	var s Stats
	var threshold float64

	dsn := fmt.Sprintf("host=localhost user=%s password=%s dbname=T00 port=5432 sslmode=disable TimeZone=Europe/Moscow", string(login), string(password))
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	// the AutoMigrate function is called with the Entry struct as its argument,
	// if the Entry table does not exist, AutoMigrate will create it.
	error := db.AutoMigrate(&Entry{})
	if error != nil {
		fmt.Println("Error at the AutoMigrate")
	}
	fmt.Println("Processing of messages has commenced")

	for {
		entry := getEntry()
		entry, err = stream.Recv()
		if err != nil {
			log.Fatalf("Error while receiving stream: %v", err)
		}

		if s.count < 100 {
			s.Approximate(entry.Frequency)
			putEntry(entry)
			continue
		} else if s.count == 100 {
			threshold = *k * math.Sqrt(s.std/2.0)
		}

		if math.Abs(entry.Frequency-s.mean) > threshold {
			e := Entry{Session_id: entry.SessionId, Frequency: entry.Frequency, Timestamp: entry.Timestamp.AsTime()}
			result := db.Create(&e)
			if result.Error != nil {
				log.Fatalf("Error while inserting entry into database: %v", result.Error)
			}
			// For loging on stage of 01 Task:
			// log.Printf("Anomalie detected: session_id: %s, frequency: %f, timestamp: %v", entry.SessionId, protobufEntry.Frequency, protobufEntry.Timestamp)
		}
		putEntry(entry)
	}

	//  To see content of our db:

	// var entries []Entry
	// db.Find(&entries)
	// for _, entry := range entries {
	// 	fmt.Println(entry)
	// }
}
