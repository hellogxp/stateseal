// Package protocol defines StateSeal's stable, serializable domain objects.
package protocol

import "time"

const Version = "v0alpha1"

type Verdict string

const (
	VerdictAdmitted  Verdict = "ADMITTED"
	VerdictRejected  Verdict = "REJECTED"
	VerdictAbstained Verdict = "ABSTAINED"
	VerdictStale     Verdict = "STALE"
	VerdictEscalated Verdict = "ESCALATED"
)

type CandidateState struct {
	CandidateID      string    `json:"candidate_id"`
	TaskID           string    `json:"task_id"`
	BaseTreeSHA256   string    `json:"base_tree_sha256"`
	PatchSHA256      string    `json:"patch_sha256"`
	ResultTreeSHA256 string    `json:"result_tree_sha256"`
	Commit           string    `json:"commit,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	Source           string    `json:"source"`
}

type EvidenceEnvelope struct {
	EvidenceID        string    `json:"evidence_id"`
	CandidateID       string    `json:"candidate_id"`
	CodeTreeSHA256    string    `json:"code_tree_sha256"`
	SuiteSHA256       string    `json:"suite_sha256"`
	CommandDigest     string    `json:"command_digest"`
	CWDigest          string    `json:"cwd_digest"`
	EnvironmentDigest string    `json:"environment_digest"`
	PolicyDigest      string    `json:"policy_digest"`
	VerifierIdentity  string    `json:"verifier_identity"`
	ExecutionID       string    `json:"execution_id"`
	StartedAt         time.Time `json:"started_at"`
	FinishedAt        time.Time `json:"finished_at"`
	ExitCode          int       `json:"exit_code"`
	ResultDigest      string    `json:"result_digest"`
	Output            string    `json:"output,omitempty"`
	TimedOut          bool      `json:"timed_out,omitempty"`
}

type VerifiedCheckpoint struct {
	CheckpointID      string    `json:"checkpoint_id"`
	CandidateID       string    `json:"candidate_id"`
	TreeSHA256        string    `json:"tree_sha256"`
	Commit            string    `json:"commit"`
	AdmissionEvidence []string  `json:"admission_evidence"`
	PolicyDigest      string    `json:"policy_digest"`
	VerifiedAt        time.Time `json:"verified_at"`
}

type CompletionReceipt struct {
	ReceiptVersion     string    `json:"receipt_version"`
	ReceiptID          string    `json:"receipt_id"`
	TaskID             string    `json:"task_id"`
	Verdict            Verdict   `json:"verdict"`
	CheckpointID       string    `json:"checkpoint_id,omitempty"`
	TreeSHA256         string    `json:"tree_sha256,omitempty"`
	CompletionEvidence []string  `json:"completion_evidence,omitempty"`
	PolicyDigest       string    `json:"policy_digest"`
	IssuedBy           string    `json:"issued_by"`
	IssuedAt           time.Time `json:"issued_at"`
	ResidualRisks      []string  `json:"residual_risks"`
	Reason             string    `json:"reason,omitempty"`
	ReceiptDigest      string    `json:"receipt_digest"`
}

type Event struct {
	Sequence  uint64         `json:"sequence"`
	Type      string         `json:"type"`
	TaskID    string         `json:"task_id"`
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
	PrevHash  string         `json:"prev_hash,omitempty"`
	Hash      string         `json:"hash"`
}

type TaskState struct {
	Version      string              `json:"version"`
	TaskID       string              `json:"task_id"`
	RepoRoot     string              `json:"repo_root"`
	BaseCommit   string              `json:"base_commit"`
	ProposalPath string              `json:"proposal_path,omitempty"`
	Mode         string              `json:"mode"`
	Status       string              `json:"status"`
	Coverage     string              `json:"checkpoint_coverage"`
	Candidate    *CandidateState     `json:"candidate,omitempty"`
	Checkpoint   *VerifiedCheckpoint `json:"checkpoint,omitempty"`
	Receipt      *CompletionReceipt  `json:"receipt,omitempty"`
	Evidence     []EvidenceEnvelope  `json:"evidence,omitempty"`
	LastError    string              `json:"last_error,omitempty"`
	Freshness    string              `json:"freshness,omitempty"`
	StaleReason  string              `json:"stale_reason,omitempty"`
	UpdatedAt    time.Time           `json:"updated_at"`
}
