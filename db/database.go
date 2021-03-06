package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/pulsar-go/pulsar/config"

	// The following imports are for the database drivers.
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

// Model represents the base database model.
type Model struct {
	ID        uint       `json:"id,omitempty" gorm:"primary_key"`
	CreatedAt time.Time  `json:"created_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" sql:"index"`
}

// DB represents the database structure used
type DB struct {
	*gorm.DB
}

// Builder represents the current database used.
var Builder *DB

// Models stores the current set of application models.
var Models []interface{}

// AddModels add the given models to the model list.
func AddModels(models ...interface{}) {
	Models = append(Models, models...)
}

// Open opens a new database connection.
func Open() {
	// Create the arguments
	var args string
	// Copy to reduce code size.
	s := &config.Settings.Database
	switch s.Driver {
	case "mysql":
		args = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", s.User, s.Password, s.Host, s.Port, s.Database)
	case "postgres":
		args = fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s", s.Host, s.Port, s.User, s.Database, s.Password)
	case "sqlite3":
		f, err := filepath.Abs(filepath.Dir(os.Args[0]) + "/" + s.Database)
		if err != nil {
			log.Fatalf("Unable to get path of database %s\n", s.Database)
		}
		args = f
	default:
		log.Fatalf("Database driver '%s' is not supported.\n", s.Driver)
	}
	// Open the database
	dbOpened, err := gorm.Open(s.Driver, args)
	if err != nil {
		log.Fatalln(err)
	}

	Builder = &DB{dbOpened}
}

// clone creates a new instance of the DB
func (b *DB) clone(lib *gorm.DB) *DB {
	return &DB{
		lib,
	}
}
