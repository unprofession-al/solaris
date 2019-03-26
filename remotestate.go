package main

type RemoteState struct {
	InFile  string
	Name    string
	Bucket  string
	Key     string
	Profile string
	Region  string
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
