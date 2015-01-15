package main

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
)

// Pipeline is a set of steps to run, this is the interface shared by
// both Build and Deploy
type Pipeline interface {
	// Getters
	Env() *Environment // base
	Steps() []*Step    // base

	// Methods
	CommonEnv() [][]string   // base
	MirrorEnv() [][]string   // base
	PassthruEnv() [][]string // base
	InitEnv()                // impl
	FetchSteps() error
	CollectArtifact(*Session) (*Artifact, error)
	SetupGuest(*Session) error
	ExportEnvironment(*Session) error

	LogEnvironment()
	DockerRepo() string
	DockerTag() string
	DockerMessage() string
}

type BasePipeline struct {
	options *GlobalOptions
	env     *Environment
	steps   []*Step
}

func NewBasePipeline(options *GlobalOptions, steps []*Step) *BasePipeline {
	return &BasePipeline{options, &Environment{}, steps}
}

func (p *BasePipeline) Steps() []*Step {
	return p.steps
}

func (p *BasePipeline) Env() *Environment {
	return p.env
}

// CommonEnv is shared by both builds and deploys
func (p *BasePipeline) CommonEnv() [][]string {
	a := [][]string{
		[]string{"WERCKER", "true"},
		[]string{"WERCKER_ROOT", p.options.GuestPath("source")},
		[]string{"WERCKER_SOURCE_DIR", p.options.GuestPath("source", p.options.SourceDir)},
		// TODO(termie): Support cache dir
		[]string{"WERCKER_CACHE_DIR", "/cache"},
		[]string{"WERCKER_OUTPUT_DIR", p.options.GuestPath("output")},
		[]string{"WERCKER_PIPELINE_DIR", p.options.GuestPath()},
		[]string{"WERCKER_REPORT_DIR", p.options.GuestPath("report")},
		[]string{"WERCKER_APPLICATION_ID", p.options.ApplicationID},
		[]string{"WERCKER_APPLICATION_NAME", p.options.ApplicationName},
		[]string{"WERCKER_APPLICATION_OWNER_NAME", p.options.ApplicationOwnerName},
		[]string{"WERCKER_APPLICATION_URL", fmt.Sprintf("%s#application/%s", p.options.BaseURL, p.options.ApplicationID)},
		//[]string{"WERCKER_STARTED_BY", ...},
		[]string{"TERM", "xterm-256color"},
	}
	return a
}

func (p *BasePipeline) MirrorEnv() [][]string {
	return p.options.Env.getMirror()
}

func (p *BasePipeline) PassthruEnv() [][]string {
	return p.options.Env.getPassthru()
}

// FetchSteps makes sure we have all the steps
func (p *BasePipeline) FetchSteps() error {
	for _, step := range p.steps {
		log.Println("Fetching Step:", step.Name, step.ID)
		if _, err := step.Fetch(); err != nil {
			return err
		}
	}
	return nil
}

// SetupGuest ensures that the guest is prepared to run the pipeline.
func (p *BasePipeline) SetupGuest(sess *Session) error {
	sess.HideLogs()
	defer sess.ShowLogs()

	cmds := []string{
		// Make sure our guest path exists
		fmt.Sprintf(`mkdir "%s"`, p.options.GuestPath()),
		// Make sure the output path exists
		fmt.Sprintf(`mkdir "%s"`, p.options.GuestPath("output")),
		// Make sure the cachedir exists
		fmt.Sprintf(`mkdir "%s"`, "/cache"),
		// Copy the source from the mounted directory to the pipeline dir
		fmt.Sprintf(`cp -r "%s" "%s"`, p.options.MntPath("source"), p.options.GuestPath("source")),
	}

	for _, cmd := range cmds {
		exit, _, err := sess.SendChecked(cmd)
		if err != nil {
			return err
		}
		if exit != 0 {
			return fmt.Errorf("Geust command failed: %s", cmd)
		}
	}

	return nil
}

// ExportEnvironment to the session
func (p *BasePipeline) ExportEnvironment(sess *Session) error {
	exit, _, err := sess.SendChecked(p.env.Export()...)
	if err != nil {
		return err
	}
	if exit != 0 {
		return fmt.Errorf("Build failed with exit code: %d", exit)
	}
	return nil
}

// LogEnvironment dumps the base environment to our logs
func (p *BasePipeline) LogEnvironment() {
	// Some helpful logging
	log.Println("Base Pipeline Environment:")
	for _, pair := range p.env.Ordered() {
		log.Println(" ", pair[0], pair[1])
	}
}