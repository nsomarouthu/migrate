package migrate

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Define a constant for the migration file name
const lastSuccessfulMigrationFile = "lastSuccessfulMigration"

func (m *Migrate) HandleDirtyState(path string) {
	// Perform actions when dirty is true
	lastSuccessfulMigrationPath := filepath.Join(path, lastSuccessfulMigrationFile)
	lastVersionBytes, err := os.ReadFile(lastSuccessfulMigrationPath)
	if err != nil {
		log.Fatalf("failed to read last successful migration file: %s", err)
	}
	lastVersionStr := strings.TrimSpace(string(lastVersionBytes))
	lastVersion, err := strconv.ParseUint(lastVersionStr, 10, 64)
	if err != nil {
		log.Fatalf("failed to parse last successful migration version: %s", err)
	}

	if err := m.Force(int(lastVersion)); err != nil {
		log.Fatalf("failed to apply last successful migration: %s", err)
	}

	log.Println("Last successful migration applied")

	if err := os.Remove(lastSuccessfulMigrationPath); err != nil {
		log.Fatalf("failed to delete last successful migration file: %s", err)
	}

	log.Println("Last successful migration file deleted")
}

func (m *Migrate) HandleMigrationFailure(curVersion int, v uint, path string) error {
	if err := m.lock(); err != nil {
		return m.unlockErr(err)
	}

	ret := make(chan interface{}, m.PrefetchMigrations)
	go m.read(curVersion, int(v), ret)

	var migrations []int
	for r := range ret {
		migrations = append(migrations, int(r.(*Migration).Version))
	}

	failedVersion, _, err := m.databaseDrv.Version()
	if err != nil {
		return err
	}
	log.Println("failedVersion:", failedVersion, "migrations:", migrations, curVersion, v)

	// Determine the last successful migration
	lastSuccessfulMigration := strconv.Itoa(curVersion)
	for i := len(migrations) - 1; i > 0; i-- { // Iterate backwards for efficiency
		if uint(migrations[i]) == uint(failedVersion) && i > 0 {
			lastSuccessfulMigration = strconv.Itoa(migrations[i-1])
			break
		}
	}

	log.Println("migration failed, last successful migration version:", lastSuccessfulMigration)
	lastSuccessfulMigrationPath := filepath.Join(path, lastSuccessfulMigrationFile)
	if err := os.WriteFile(lastSuccessfulMigrationPath, []byte(lastSuccessfulMigration), 0644); err != nil {
		return err
	}

	return nil
}

func (m *Migrate) CleanupFiles(path string, v uint) error {
	files, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	targetVersion := uint64(v)

	for _, file := range files {
		fileName := file.Name()

		// Check if file is a migration file we want to process
		if !strings.HasSuffix(fileName, "down.sql") && !strings.HasSuffix(fileName, "up.sql") {
			continue
		}

		// Extract version and compare
		versionEnd := strings.Index(fileName, "_")
		if versionEnd == -1 {
			// Skip files that don't match the expected naming pattern
			continue
		}

		fileVersion, err := strconv.ParseUint(fileName[:versionEnd], 10, 64)
		if err != nil {
			log.Printf("Skipping file %s due to version parse error: %v", fileName, err)
			continue
		}

		// Delete file if version is greater than targetVersion
		if fileVersion > targetVersion {
			if err := os.Remove(filepath.Join(path, fileName)); err != nil {
				log.Printf("Failed to delete file %s: %v", fileName, err)
				continue
			}
			log.Printf("Deleted file: %s", fileName)
		}
	}

	return nil
}
