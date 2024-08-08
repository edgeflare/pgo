package logrepl

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	outputPlugin        = "pgoutput"
	publicationName     = "pglogrepl_demo"
	slotName            = "pglogrepl_demo"
	standbyMessageTimer = 10 * time.Second
)

func SetupReplication() (*pgconn.PgConn, pglogrepl.IdentifySystemResult, error) {
	connString := os.Getenv("PGO_PGLOGREPL_CONN_STRING")
	conn, err := pgconn.Connect(context.Background(), connString)
	if err != nil {
		return nil, pglogrepl.IdentifySystemResult{}, err
	}

	exists, err := checkPublicationExists(conn, publicationName)
	if err != nil {
		conn.Close(context.Background())
		return nil, pglogrepl.IdentifySystemResult{}, err
	}
	if !exists {
		err = createPublication(conn, publicationName)
		if err != nil {
			conn.Close(context.Background())
			return nil, pglogrepl.IdentifySystemResult{}, err
		}
		log.Println("Created publication", publicationName)
	} else {
		log.Println("Publication", publicationName, "already exists")
	}

	sysident, err := pglogrepl.IdentifySystem(context.Background(), conn)
	if err != nil {
		conn.Close(context.Background())
		return nil, pglogrepl.IdentifySystemResult{}, err
	}
	log.Println("SystemID:", sysident.SystemID, "Timeline:", sysident.Timeline, "XLogPos:", sysident.XLogPos, "DBName:", sysident.DBName)

	slotExists, err := checkSlotExists(conn, slotName)
	if err != nil {
		conn.Close(context.Background())
		return nil, pglogrepl.IdentifySystemResult{}, err
	}
	if !slotExists {
		err = createReplicationSlot(conn, slotName, outputPlugin)
		if err != nil {
			conn.Close(context.Background())
			return nil, pglogrepl.IdentifySystemResult{}, err
		}
		log.Println("Created replication slot", slotName)
	} else {
		log.Println("Replication slot", slotName, "already exists")
	}

	return conn, sysident, nil
}

func checkPublicationExists(conn *pgconn.PgConn, publicationName string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = '%s');", publicationName)
	result := conn.Exec(context.Background(), query)
	rows, err := result.ReadAll()
	if err != nil {
		return false, err
	}
	if len(rows) > 0 && len(rows[0].Rows) > 0 {
		return string(rows[0].Rows[0][0]) == "t", nil
	}
	return false, nil
}

func createPublication(conn *pgconn.PgConn, publicationName string) error {
	query := fmt.Sprintf("CREATE PUBLICATION %s FOR ALL TABLES;", publicationName)
	result := conn.Exec(context.Background(), query)
	_, err := result.ReadAll()
	return err
}

func checkSlotExists(conn *pgconn.PgConn, slotName string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM pg_replication_slots WHERE slot_name = '%s');", slotName)
	result := conn.Exec(context.Background(), query)
	rows, err := result.ReadAll()
	if err != nil {
		return false, err
	}
	if len(rows) > 0 && len(rows[0].Rows) > 0 {
		return string(rows[0].Rows[0][0]) == "t", nil
	}
	return false, nil
}

func createReplicationSlot(conn *pgconn.PgConn, slotName string, outputPlugin string) error {
	_, err := pglogrepl.CreateReplicationSlot(context.Background(), conn, slotName, outputPlugin, pglogrepl.CreateReplicationSlotOptions{Temporary: false})
	return err
}
