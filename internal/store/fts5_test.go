package store

import (
	"context"
	"testing"
)

// TestFTS5_AvailableAndUsable is the proof required by AGENTS.md before the
// SQLite choice is final: the pure-Go driver must support FTS5, including
// prefix queries, case-insensitive matching and bm25 ranking. It also
// exercises the query shape the search service will build from user input:
// quoted tokens with a trailing * for prefix matching (never raw input).
func TestFTS5_AvailableAndUsable(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ctx := context.Background()

	_, err := db.ExecContext(ctx, `
		CREATE VIRTUAL TABLE search USING fts5(
			title,
			description,
			note,
			tags,
			tokenize = 'unicode61 remove_diacritics 2'
		);
		INSERT INTO search (rowid, title, description, note, tags)
			VALUES (1, 'Rust Ownership', 'memory safety', 'the borrow checker explained', 'rust programming');
		INSERT INTO search (rowid, title, description, note, tags)
			VALUES (2, 'Cooking', 'dinner ideas', 'rust never sleeps', 'food');
	`)
	if err != nil {
		t.Fatalf("FTS5 is not usable with this driver: %v", err)
	}

	tests := []struct {
		name  string
		query string
		want  []int
	}{
		{name: "prefix", query: `"own"*`, want: []int{1}},
		{name: "case insensitive", query: `"BORROW"*`, want: []int{1}},
		{name: "multi-word all required", query: `"rust"* "checker"*`, want: []int{1}},
		{name: "no match", query: `"rust"* "zebra"*`, want: nil},
		{name: "token prefix, not substring", query: `"own"`, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := db.QueryContext(ctx,
				`SELECT rowid FROM search WHERE search MATCH ? ORDER BY rowid`, tt.query)
			if err != nil {
				t.Fatalf("querying %q: %v", tt.query, err)
			}
			defer rows.Close() //nolint:errcheck // read-only query

			var got []int
			for rows.Next() {
				var id int
				if err := rows.Scan(&id); err != nil {
					t.Fatalf("scanning rowid: %v", err)
				}
				got = append(got, id)
			}
			if err := rows.Err(); err != nil {
				t.Fatalf("iterating results: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("query %q matched %v, want %v", tt.query, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("query %q matched %v, want %v", tt.query, got, tt.want)
				}
			}
		})
	}

	// bm25 weights per column: title, description, note, tags. A title match
	// must outrank a note-only match (spec S-3).
	rows, err := db.QueryContext(ctx,
		`SELECT rowid FROM search WHERE search MATCH '"rust"*'
		 ORDER BY bm25(search, 10.0, 3.0, 1.0, 5.0), rowid`)
	if err != nil {
		t.Fatalf("bm25 ranking query: %v", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	var ranked []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scanning ranked rowid: %v", err)
		}
		ranked = append(ranked, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating ranked results: %v", err)
	}
	if len(ranked) != 2 || ranked[0] != 1 {
		t.Fatalf("bm25 order = %v, want the title match (rowid 1) first", ranked)
	}
}
