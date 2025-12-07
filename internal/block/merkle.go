package block

import (
	"crypto/sha256"
	"encoding/hex"
)

// ComputeMerkleRoot builds a Merkle root from a slice of transaction IDs.
// If only one tx, its hash is the root. Otherwise, pairwise hash up the tree.
func ComputeMerkleRoot(txIDs []string) string {
	if len(txIDs) == 0 {
		return ""
	}

	// Hash each tx_id initially
	var level []string
	for _, id := range txIDs {
		sum := sha256.Sum256([]byte(id))
		level = append(level, hex.EncodeToString(sum[:]))
	}

	// Build tree
	for len(level) > 1 {
		var next []string
		for i := 0; i < len(level); i += 2 {
			if i+1 < len(level) {
				combined := level[i] + level[i+1]
				sum := sha256.Sum256([]byte(combined))
				next = append(next, hex.EncodeToString(sum[:]))
			} else {
				// odd count: duplicate last
				combined := level[i] + level[i]
				sum := sha256.Sum256([]byte(combined))
				next = append(next, hex.EncodeToString(sum[:]))
			}
		}
		level = next
	}
	return level[0]
}
