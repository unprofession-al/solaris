package main

type RemoteState struct {
	InFile  string `json:"in_file"`
	Name    string `json:"name"`
	Bucket  string `json:"bucket"`
	Key     string `json:"key"`
	Profile string `json:"profile"`
	Region  string `json:"region"`
}

func (orig RemoteState) equals(other RemoteState) bool {
	if orig.Bucket == other.Bucket &&
		orig.Key == other.Key &&
		orig.Profile == other.Profile &&
		orig.Region == other.Region {
		return true
	}
	return false
}
