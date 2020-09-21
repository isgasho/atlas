package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Module defines a Cosmos SDK module.
type Module struct {
	gorm.Model

	Name          string `gorm:"not null;default:null" json:"name" yaml:"name"`
	Team          string `gorm:"not null;default:null" json:"team" yaml:"team"`
	Description   string `json:"description" yaml:"description"`
	Documentation string `json:"documentation" yaml:"documentation"`
	Homepage      string `json:"homepage" yaml:"homepage"`
	Repo          string `gorm:"not null;default:null" json:"repo" yaml:"repo"`

	// one-to-one relationships
	BugTracker BugTracker `json:"bug_tracker" yaml:"bug_tracker" gorm:"foreignKey:module_id"`

	// many-to-many relationships
	Keywords []Keyword `gorm:"many2many:module_keywords" json:"keywords" yaml:"keywords"`
	Authors  []User    `gorm:"many2many:module_authors" json:"authors" yaml:"authors"`

	// one-to-many relationships
	Version  string          `gorm:"-" json:"-" yaml:"-"` // current version in manifest
	Versions []ModuleVersion `gorm:"foreignKey:module_id" json:"versions" yaml:"versions"`
}

// Upsert will attempt to either create a new Module record or update an
// existing record. A Module record is considered unique by a (name, team) index.
// In the case of the record existing, all primary and one-to-one fields will be
// updated, where authors and keywords are replaced. If the provided Version
// does not exist, it will be appended to the existing set of version relations.
// An error is returned upon failure. Upon success, the created or updated record
// will be returned.
func (m Module) Upsert(db *gorm.DB) (Module, error) {
	var record Module

	tx := db.Where("name = ? AND team = ?", m.Name, m.Team).First(&record)
	if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		if m.Version == "" {
			return Module{}, errors.New("failed to create module: empty module version")
		}
		if len(m.Authors) == 0 {
			return Module{}, errors.New("failed to create module: empty module authors")
		}

		// record does not exist, so we create it
		if err := db.Create(&m).Error; err != nil {
			return Module{}, fmt.Errorf("failed to create module: %w", err)
		}

		return m, nil
	}

	// record exists, so we update the relevant fields
	tx = db.Preload(clause.Associations).First(&record)

	// retrieve or create all authors and update the association
	for i, u := range m.Authors {
		if err := db.Where(User{Name: u.Name}).FirstOrCreate(&u).Error; err != nil {
			return Module{}, fmt.Errorf("failed to fetch or create author: %w", err)
		}
		m.Authors[i] = u
	}

	if err := db.Model(&record).Association("Authors").Replace(m.Authors); err != nil {
		return Module{}, fmt.Errorf("failed to update module authors: %w", err)
	}

	// retrieve or create all keywords and update the association
	for i, k := range m.Keywords {
		if err := db.Where(Keyword{Name: k.Name}).FirstOrCreate(&k).Error; err != nil {
			return Module{}, fmt.Errorf("failed to fetch or create keyword: %w", err)
		}
		m.Keywords[i] = k
	}

	if err := db.Model(&record).Association("Keywords").Replace(m.Keywords); err != nil {
		return Module{}, fmt.Errorf("failed to update module keywords: %w", err)
	}

	// update the bug tracker association
	if err := db.Model(&record.BugTracker).Updates(m.BugTracker).Error; err != nil {
		return Module{}, fmt.Errorf("failed to update module bug tracker: %w", err)
	}

	// append version if new
	versionQuery := &ModuleVersion{Version: m.Version, ModuleID: record.ID}
	if err := db.Where(versionQuery).First(&ModuleVersion{}).Error; err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		if err := db.Model(&record).Association("Versions").Append(&ModuleVersion{Version: m.Version}); err != nil {
			return Module{}, fmt.Errorf("failed to update module version: %w", err)
		}
	}

	// update primary fields
	if err := tx.Updates(Module{
		Team:          m.Team,
		Description:   m.Description,
		Documentation: m.Documentation,
		Homepage:      m.Homepage,
		Repo:          m.Repo,
	}).Error; err != nil {
		return Module{}, fmt.Errorf("failed to update module: %w", err)
	}

	return record, nil
}
