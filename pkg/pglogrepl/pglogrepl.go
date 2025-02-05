package pglogrepl

import (
	"cmp"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
)

// Config holds the configuration for the replication process.
type Config struct {
	// ConnString is the PostgreSQL connection string.
	ConnString string

	// PublicationName is the name of the publication to be used or created.
	PublicationName string

	// SlotName is the name of the replication slot to be used or created.
	SlotName string

	// OutputPlugin specifies the output plugin to be used for replication.
	OutputPlugin string

	// StandbyMessageTimeout is the duration between standby status messages.
	StandbyMessageTimeout time.Duration
}

// replication config
var (
	outputPlugin    = cmp.Or(os.Getenv("PGO_LOGREPL_OUTPUT_PLUGIN"), "pgoutput") // or wal2json. prefer pgoutput for performance
	publicationName = cmp.Or(os.Getenv("PGO_LOGREPL_PUBLICATION_NAME"), "pgo_logrepl")
	slotName        = cmp.Or(os.Getenv("PGO_LOGREPL_SLOT_NAME"), "pgo_logrepl")
)

// SetupReplication initializes the replication process by connecting to the database,
// creating a publication if it doesn't exist, and setting up a replication slot.
// It returns a database connection, system identification information, and any error encountered.
func SetupReplication(config Config) (*pgconn.PgConn, pglogrepl.IdentifySystemResult, error) {
	conn, err := pgconn.Connect(context.Background(), config.ConnString)
	if err != nil {
		return nil, pglogrepl.IdentifySystemResult{}, err
	}

	exists, err := checkPublicationExists(conn, config.PublicationName)
	if err != nil {
		conn.Close(context.Background())
		return nil, pglogrepl.IdentifySystemResult{}, err
	}
	if !exists {
		err = createPublication(conn, config.PublicationName)
		if err != nil {
			conn.Close(context.Background())
			return nil, pglogrepl.IdentifySystemResult{}, err
		}
		log.Println("Created publication", config.PublicationName)
	} else {
		log.Println("Publication", config.PublicationName, "already exists")
	}

	sysident, err := pglogrepl.IdentifySystem(context.Background(), conn)
	if err != nil {
		conn.Close(context.Background())
		return nil, pglogrepl.IdentifySystemResult{}, err
	}
	log.Println("SystemID:", sysident.SystemID, "Timeline:", sysident.Timeline, "XLogPos:", sysident.XLogPos, "DBName:", sysident.DBName)

	slotExists, err := checkSlotExists(conn, config.SlotName)
	if err != nil {
		conn.Close(context.Background())
		return nil, pglogrepl.IdentifySystemResult{}, err
	}
	if !slotExists {
		err = createReplicationSlot(conn, config.SlotName, config.OutputPlugin)
		if err != nil {
			conn.Close(context.Background())
			return nil, pglogrepl.IdentifySystemResult{}, err
		}
		log.Println("Created replication slot", config.SlotName)
	} else {
		log.Println("Replication slot", config.SlotName, "already exists")
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
	// Create the publication without specifying any tables
	query := fmt.Sprintf("CREATE PUBLICATION %s;", publicationName)
	result := conn.Exec(context.Background(), query)
	_, err := result.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to create publication: %w", err)
	}

	return nil
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

// addTableToPublication adds a specified table to an existing publication.
func addTableToPublication(conn *pgconn.PgConn, publicationName string, schemaName string, tableName string) error {
	query := fmt.Sprintf("ALTER PUBLICATION %s ADD TABLE %s.%s;", publicationName, schemaName, tableName)
	result := conn.Exec(context.Background(), query)
	_, err := result.ReadAll()
	return err
}
