package juju

type StatusRequest struct {
	Model     string
	Relations bool
	Storage   bool
}

func StatusArgs(req StatusRequest) []string {
	args := []string{"status", "-m", req.Model, "--format=json"}
	if req.Relations {
		args = append(args, "--relations")
	}
	if req.Storage {
		args = append(args, "--storage")
	}
	return args
}
