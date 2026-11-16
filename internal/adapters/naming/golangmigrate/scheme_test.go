package golangmigrate_test

import (
	"errors"
	"testing"

	"github.com/bSemir/migraine/internal/adapters/naming/golangmigrate"
	"github.com/bSemir/migraine/internal/core/domain"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     domain.Part
		wantErr  error
	}{
		{"up", "000042_add_users.up.sql", domain.Part{Version: 42, Width: 6, Title: "add_users", Suffix: ".up.sql"}, nil},
		{"down", "000042_add_users.down.sql", domain.Part{Version: 42, Width: 6, Title: "add_users", Suffix: ".down.sql"}, nil},
		{"unpadded", "7_x.up.sql", domain.Part{Version: 7, Width: 1, Title: "x", Suffix: ".up.sql"}, nil},
		{"title with underscores and digits", "003_add_2fa_to_users_v2.up.sql", domain.Part{Version: 3, Width: 3, Title: "add_2fa_to_users_v2", Suffix: ".up.sql"}, nil},
		{"title starting with digits", "003_2024_backfill.down.sql", domain.Part{Version: 3, Width: 3, Title: "2024_backfill", Suffix: ".down.sql"}, nil},
		{"title with dots", "003_v1.2_schema.up.sql", domain.Part{Version: 3, Width: 3, Title: "v1.2_schema", Suffix: ".up.sql"}, nil},
		{"nine digits is still sequential", "123456789_x.up.sql", domain.Part{Version: 123456789, Width: 9, Title: "x", Suffix: ".up.sql"}, nil},

		{"no direction", "000042_add_users.sql", domain.Part{}, domain.ErrNotMigration},
		{"no version", "add_users.up.sql", domain.Part{}, domain.ErrNotMigration},
		{"no title", "000042_.up.sql", domain.Part{}, domain.ErrNotMigration},
		{"no separator", "000042.up.sql", domain.Part{}, domain.ErrNotMigration},
		{"wrong extension", "000042_add_users.up.txt", domain.Part{}, domain.ErrNotMigration},
		{"unrelated file", "README.md", domain.Part{}, domain.ErrNotMigration},
		{"empty", "", domain.Part{}, domain.ErrNotMigration},

		{"timestamp", "20240101120000_x.up.sql", domain.Part{}, domain.ErrTimestampVersion},
		{"timestamp without direction", "20240101120000_x.sql", domain.Part{}, domain.ErrTimestampVersion},
		{"unix timestamp", "1704110400_x.up.sql", domain.Part{}, domain.ErrTimestampVersion},
	}
	scheme := golangmigrate.New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scheme.Parse(tt.filename)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Parse(%q) error = %v, want %v", tt.filename, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestFormatRoundTrip(t *testing.T) {
	scheme := golangmigrate.New()
	for _, name := range []string{"000042_add_users.up.sql", "7_x.down.sql", "0999_a_b_3.up.sql"} {
		part, err := scheme.Parse(name)
		if err != nil {
			t.Fatalf("Parse(%q): %v", name, err)
		}
		if got := scheme.Format(part); got != name {
			t.Errorf("Format(Parse(%q)) = %q", name, got)
		}
	}
}

func TestFormatWidensWhenVersionOutgrowsWidth(t *testing.T) {
	got := golangmigrate.New().Format(domain.Part{Version: 1000, Width: 3, Title: "x", Suffix: ".up.sql"})
	if want := "1000_x.up.sql"; got != want {
		t.Errorf("Format = %q, want %q", got, want)
	}
}
