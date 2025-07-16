package service

type RunCommandRequest struct {
	Command string `json:"command"`
}

type RunCommandResponse struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	Code   int    `json:"code"`
	Error  string `json:"error,omitempty"`
}
