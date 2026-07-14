package dto

type ThreeDMetadata struct {
	ArtStyle string `json:"art_style,omitempty"`
}

type ThreeDRequest struct {
	Model          string          `json:"model"`
	Prompt         string          `json:"prompt,omitempty"`
	InputReference string          `json:"input_reference,omitempty"`
	SourceTaskID   string          `json:"source_task_id,omitempty"`
	Metadata       *ThreeDMetadata `json:"metadata,omitempty"`
}

type ThreeDData struct {
	Format string `json:"format"`
	URL    string `json:"url"`
}

type ThreeDError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type ThreeDResponse struct {
	ID          string       `json:"id"`
	Object      string       `json:"object"`
	Model       string       `json:"model"`
	Status      string       `json:"status"`
	Progress    int          `json:"progress"`
	CreatedAt   int64        `json:"created_at"`
	CompletedAt int64        `json:"completed_at,omitempty"`
	Data        *ThreeDData  `json:"data,omitempty"`
	Error       *ThreeDError `json:"error,omitempty"`
}
