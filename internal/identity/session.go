package identity

import "io"

type SessionBinding struct {
	Workspace  string `json:"workspace"`
	ProfileID  string `json:"profileID"`
	SessionID  string `json:"sessionID"`
	Generation uint64 `json:"generation"`
}

func NewSessionID(random io.Reader) (string, error) {
	return randomID("session-", random)
}

func (r *Registry) UpsertBinding(value SessionBinding) {
	for index := range r.Bindings {
		if r.Bindings[index].Workspace == value.Workspace && r.Bindings[index].ProfileID == value.ProfileID {
			r.Bindings[index] = value
			return
		}
	}
	r.Bindings = append(r.Bindings, value)
}

func (r *Registry) RemoveBinding(workspace, profileID string) {
	result := r.Bindings[:0]
	for _, value := range r.Bindings {
		if value.Workspace != workspace || value.ProfileID != profileID {
			result = append(result, value)
		}
	}
	r.Bindings = result
}

func (r Registry) Binding(workspace, profileID string) (SessionBinding, bool) {
	for _, value := range r.Bindings {
		if value.Workspace == workspace && value.ProfileID == profileID {
			return value, true
		}
	}
	return SessionBinding{}, false
}
