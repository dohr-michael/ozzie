package skills

// TriggersDef describes scheduling and activation triggers loaded from triggers.yaml.
type TriggersDef struct {
	Delegation bool          `yaml:"delegation"`
	Cron       string        `yaml:"cron,omitempty"`
	OnEvent    *EventTrigger `yaml:"on_event,omitempty"`
	Keywords   []string      `yaml:"keywords,omitempty"`
}

// HasScheduleTrigger returns true if the triggers include a cron or event trigger.
func (td *TriggersDef) HasScheduleTrigger() bool {
	return td != nil && (td.Cron != "" || td.OnEvent != nil)
}
