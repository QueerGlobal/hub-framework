package helpers

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"

	configuration "github.com/QueerGlobal/qg-config-go/configuration"
	_ "github.com/go-sql-driver/mysql"
)

type DB struct {
	Writer *sql.DB
	Reader *sql.DB
}

var dbInstance *DB

func GetDB() *DB {
	if dbInstance == nil {
		fmt.Println("Building db instance...")
		err := NewDB()
		if err != nil {
			log.Println(err)
		}
	}
	return dbInstance
}

func NewDB() error {

	database := DB{}
	initPath := "init.json"
	config, err := configuration.GetConfig(&initPath)
	if err != nil {
		return err
	}

	dbType, err := config.GetString("amp-hub-DbType")
	if err != nil {
		return fmt.Errorf("Could not read amp-hub-DbType %w", err)
	}
	dbHost, err := config.GetString("amp-hub-DbHost")
	if err != nil {
		return fmt.Errorf("Could not read amp-hub-DbHost %w", err)
	}
	dbPort, err := config.GetString("amp-hub-DbPort")
	if err != nil {
		return fmt.Errorf("Could not read amp-hub-DbPort %w", err)
	}
	dbUser, err := config.GetString("amp-hub-DbUser")
	if err != nil {
		return fmt.Errorf("Could not read amp-hub-DbUser %w", err)
	}
	dbPassword, err := config.GetString("amp-hub-DbPw")
	if err != nil {
		return fmt.Errorf("Could not read amp-hub-DbPw %w", err)
	}
	dbSchema, err := config.GetString("amp-hub-DbSchema")
	if err != nil {
		return fmt.Errorf("Could not read amp-hub-DbSchema %w", err)
	}

	dbRWSame, err := config.GetString("amp-hub-DbReaderIsWriter")
	if err != nil {
		return fmt.Errorf("amp-hub-DbReaderIsWriter %w", err)
	}
	var connectionString string

	if *dbType == "mysql" {
		connectionString = *dbUser + ":" + *dbPassword + "@tcp(" + *dbHost + ":" + *dbPort + ")"

		if *dbSchema != "" {
			connectionString += "/" + *dbSchema
		}
	}
	connectionString += "?parseTime=true"

	db, err := sql.Open(*dbType, connectionString) //"user:password@tcp(127.0.0.1:3306)/hello")
	if err != nil {
		return err
	}
	err = db.Ping()
	if err != nil {
		return err
	}
	database.Writer = db

	same, err := strconv.ParseBool(*dbRWSame)
	if err != nil || !same {

		database.Reader = database.Writer
		fmt.Println("Could not parse DbWRSame parameter. Assuming writer and reader use same connection")

	} else {
		if same {

			database.Reader = database.Writer

		} else {

			dbReaderHost, err := config.GetString("amp-hub-DbReaderHost")
			if err != nil {
				return fmt.Errorf("Could not read amp-hub-DbReaderHost %w", err)
			}
			dbReaderPort, err := config.GetString("amp-hub-DbReaderPort")
			if err != nil {
				return fmt.Errorf("Could not read amp-hub-DbReaderPort %w", err)
			}
			dbReaderUser, err := config.GetString("amp-hub-DbReaderUser")
			if err != nil {
				return fmt.Errorf("Could not read amp-hub-DbReaderUser %w", err)
			}
			dbReaderPassword, err := config.GetString("amp-hub-DbReaderPw")
			if err != nil {
				return fmt.Errorf("Could not read amp-hub-DbReaderPw %w", err)
			}

			connectionString := *dbReaderUser + ":" + *dbReaderPassword + "@tcp(" + *dbReaderHost + ":" + *dbReaderPort + ")"

			if *dbSchema != "" {
				connectionString += "/" + *dbSchema
			}

			db, err := sql.Open(*dbType, connectionString) //"user:password@tcp(127.0.0.1:3306)/hello")
			if err != nil {
				return err
			}

			err = db.Ping()
			if err != nil {
				return err
			}
			database.Reader = db
		}
	}
	dbInstance = &database

	fmt.Print("Writer initialized: ")
	fmt.Println(dbInstance.Writer != nil)
	fmt.Print("Reader initialized: ")
	fmt.Println(dbInstance.Reader != nil)
	fmt.Println("Database initialized")
	return nil
}

func SetDBInstance(db *DB) {
	dbInstance = db
}
