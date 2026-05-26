package dockerfile

import "github.com/joacohbc/my-devcontainer-installer/cli-go/internal/core"

var DodModule = &DockerfileModuleSpec{
	ID:       "dod",
	Label:    "Docker-outside-Docker (DoD)",
	Category: core.CategoryInfra,
	Render: func(opts map[string]any) string {
		return `##
## DOCKER-OUTSIDE-DOCKER SETUP
##
RUN mkdir -p /etc/apt/keyrings
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg

RUN echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
    $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

RUN apt-get update && apt-get install -y docker-ce-cli

RUN groupadd docker || true
RUN usermod -aG docker devuser
`
	},
}
