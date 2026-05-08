package service

import (
	"log"
)

type SagaStep struct {
	Name       string
	Execute    func() error
	Compensate func() error
}

type Saga struct {
	Steps []SagaStep
}

func (s *Saga) Execute() error {
	for i, step := range s.Steps {
		log.Printf("Executing step: %s", step.Name)
		if err := step.Execute(); err != nil {
			log.Printf("Error in step: %s, initiating compensation...", step.Name)
			for j := i - 1; j >= 0; j-- {
				err := s.Steps[j].Compensate()
				if err != nil {
					return err
				}
			}
			return err
		}
	}
	return nil
}
