package commands

import "testing"

func TestParseDiffStatLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		files   int
		insert  int
		deleted int
	}{
		{
			name:   "all three clauses",
			line:   "3 files changed, 10 insertions(+), 2 deletions(-)",
			files:  3,
			insert: 10, deleted: 2,
		},
		{
			name:   "insertions only, singular file",
			line:   "1 file changed, 1 insertion(+)",
			files:  1,
			insert: 1, deleted: 0,
		},
		{
			name:   "deletions only",
			line:   "1 file changed, 5 deletions(-)",
			files:  1,
			insert: 0, deleted: 5,
		},
		{
			name:   "singular insertion and deletion",
			line:   "2 files changed, 1 insertion(+), 1 deletion(-)",
			files:  2,
			insert: 1, deleted: 1,
		},
		{
			name:   "large counts",
			line:   "12 files changed, 340 insertions(+), 128 deletions(-)",
			files:  12,
			insert: 340, deleted: 128,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := &DiffStats{}
			parseDiffStatLine(tt.line, stats)
			if stats.FilesChanged != tt.files {
				t.Errorf("FilesChanged = %d, want %d", stats.FilesChanged, tt.files)
			}
			if stats.Insertions != tt.insert {
				t.Errorf("Insertions = %d, want %d", stats.Insertions, tt.insert)
			}
			if stats.Deletions != tt.deleted {
				t.Errorf("Deletions = %d, want %d", stats.Deletions, tt.deleted)
			}
		})
	}
}
