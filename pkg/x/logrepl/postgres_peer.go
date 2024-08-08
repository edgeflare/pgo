package logrepl

import (
	"context"
)

type PostgresPeer struct {
	EventChan chan Event
	errorChan chan error
}

func NewPostgresPeer() *PostgresPeer {
	return &PostgresPeer{
		EventChan: make(chan Event),
		errorChan: make(chan error),
	}
}

func (p *PostgresPeer) Name() string {
	return "PostgresPeer"
}

func (p *PostgresPeer) Connect(ctx context.Context, config interface{}) error {
	return nil
}

func (p *PostgresPeer) Disconnect(ctx context.Context) error {
	return nil
}

func (p *PostgresPeer) Write(ctx context.Context, data Event) error {
	p.EventChan <- data
	return nil
}

func (p *PostgresPeer) BatchWrite(ctx context.Context, data []Event) error {
	for _, event := range data {
		p.EventChan <- event
	}
	return nil
}

func (p *PostgresPeer) Read(ctx context.Context) (<-chan Event, error) {
	return p.EventChan, nil
}

func (p *PostgresPeer) BatchRead(ctx context.Context, batchSize int) (<-chan []Event, error) {
	return nil, nil
}

func (p *PostgresPeer) Errors() <-chan error {
	return p.errorChan
}

func (p *PostgresPeer) Metadata() map[string]interface{} {
	return nil
}

func (p *PostgresPeer) PauseStreaming(ctx context.Context) error {
	return nil
}

func (p *PostgresPeer) ResumeStreaming(ctx context.Context) error {
	return nil
}
