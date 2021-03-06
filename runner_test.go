package nacelle

import (
	"errors"
	"sync"

	"github.com/aphistic/sweet"
	. "github.com/onsi/gomega"

	"github.com/efritz/nacelle/log"
)

type RunnerSuite struct{}

func (s *RunnerSuite) TestRunOrder(t sweet.T) {
	var (
		runner     = NewProcessRunner(NewServiceContainer())
		initChan   = make(chan string)
		startChan  = make(chan string)
		errChan    = make(chan error)
		numStopped = 0
	)

	makeProcess := func(name string) Process {
		p := &mockProcess{}
		c := make(chan struct{})

		p.init = func(config Config) error {
			initChan <- name
			return nil
		}

		p.start = func() error {
			startChan <- name
			<-c
			return nil
		}

		o := &sync.Once{}

		p.stop = func() error {
			o.Do(func() {
				numStopped++
				close(c)
			})

			return nil
		}

		return p
	}

	var (
		proc1 = makeProcess("proc1")
		proc2 = makeProcess("proc2")
		proc3 = makeProcess("proc3")
		proc4 = makeProcess("proc4")

		n1 string
		n2 string
	)

	runner.RegisterInitializer(makeProcess("init1"))
	runner.RegisterInitializer(makeProcess("init2"))
	runner.RegisterInitializer(makeProcess("init3"))
	runner.RegisterProcess(proc1, WithPriority(1))
	runner.RegisterProcess(proc2, WithPriority(2))
	runner.RegisterProcess(proc3, WithPriority(1))
	runner.RegisterProcess(proc4, WithPriority(2))

	go func() {
		defer close(errChan)

		for err := range runner.Run(nil, log.NewNilLogger()) {
			errChan <- err
		}
	}()

	//
	// Initializers run in order

	Consistently(startChan).ShouldNot(Receive())
	Eventually(initChan).Should(Receive(Equal("init1")))
	Eventually(initChan).Should(Receive(Equal("init2")))
	Eventually(initChan).Should(Receive(Equal("init3")))

	//
	// Priority 1 initializes in order

	Consistently(startChan).ShouldNot(Receive())
	Eventually(initChan).Should(Receive(Equal("proc1")))
	Eventually(initChan).Should(Receive(Equal("proc3")))

	//
	// Priority 1 starts and runs concurrently

	Eventually(startChan).Should(Receive(&n1))
	Eventually(startChan).Should(Receive(&n2))
	Expect([]string{n1, n2}).To(Or(
		Equal([]string{"proc1", "proc3"}),
		Equal([]string{"proc3", "proc1"}),
	))

	//
	// Priority 2 initializes in order

	Consistently(startChan).ShouldNot(Receive())
	Eventually(initChan).Should(Receive(Equal("proc2")))
	Eventually(initChan).Should(Receive(Equal("proc4")))

	//
	// Priority 2 starts and runs concurrently

	Eventually(startChan).Should(Receive(&n1))
	Eventually(startChan).Should(Receive(&n2))
	Expect([]string{n1, n2}).To(Or(
		Equal([]string{"proc2", "proc4"}),
		Equal([]string{"proc4", "proc2"}),
	))

	//
	// Stopping one process stops all

	Consistently(errChan).ShouldNot(BeClosed())
	proc1.Stop()
	Eventually(errChan).Should(BeClosed())
	Expect(numStopped).To(Equal(4))
}

func (s *RunnerSuite) TestRunNonBlockingProcesses(t sweet.T) {
	var (
		runner     = NewProcessRunner(NewServiceContainer())
		startChan  = make(chan string)
		errChan    = make(chan error)
		numStopped = 0
	)

	makeProcess := func(name string) Process {
		p := &mockProcess{}
		c := make(chan struct{})

		p.init = func(config Config) error {
			return nil
		}

		p.start = func() error {
			startChan <- name
			<-c
			return nil
		}

		o := &sync.Once{}

		p.stop = func() error {
			o.Do(func() {
				numStopped++
				close(c)
			})

			return nil
		}

		return p
	}

	var (
		proc1 = makeProcess("proc1")
		proc2 = makeProcess("proc2")
		proc3 = makeProcess("proc3")
		proc4 = makeProcess("proc4")
	)

	runner.RegisterProcess(proc1, WithPriority(1), WithSilentExit())
	runner.RegisterProcess(proc2, WithPriority(2), WithSilentExit())
	runner.RegisterProcess(proc3, WithPriority(1))
	runner.RegisterProcess(proc4, WithPriority(2))

	go func() {
		defer close(errChan)

		for err := range runner.Run(nil, log.NewNilLogger()) {
			errChan <- err
		}
	}()

	//
	// Wait for all processes to start

	Eventually(startChan).Should(Receive())
	Eventually(startChan).Should(Receive())
	Eventually(startChan).Should(Receive())
	Eventually(startChan).Should(Receive())

	//
	// Can shutdown processes marked with silent shutdown

	proc1.Stop()
	Consistently(errChan).ShouldNot(BeClosed())
	Expect(numStopped).To(Equal(1))

	proc2.Stop()
	Consistently(errChan).ShouldNot(BeClosed())
	Expect(numStopped).To(Equal(2))

	//
	// Other processes will stop the rest

	proc3.Stop()
	Eventually(errChan).Should(BeClosed())
	Expect(numStopped).To(Equal(4))
}

func (s *RunnerSuite) TestProcessError(t sweet.T) {
	var (
		runner   = NewProcessRunner(NewServiceContainer())
		stopChan = make(chan string)
		errChan  = make(chan error)
	)

	makeProcess := func(name string, startError, stopError error) Process {
		p := &mockProcess{}
		c := make(chan struct{})

		p.init = func(config Config) error {
			return nil
		}

		p.start = func() error {
			if startError != nil {
				return startError
			}

			<-c
			return nil
		}

		p.stop = func() error {
			stopChan <- name
			close(c)
			return stopError
		}

		return p
	}

	var (
		startError = errors.New("error in start")
		stopError  = errors.New("error in stop")

		proc1 = makeProcess("proc1", nil, nil)
		proc2 = makeProcess("proc2", nil, stopError)
		proc3 = makeProcess("proc3", nil, nil)
		proc4 = makeProcess("proc4", startError, nil)
	)

	runner.RegisterProcess(proc1, WithPriority(1))
	runner.RegisterProcess(proc2, WithPriority(2), WithProcessName("foo"))
	runner.RegisterProcess(proc3, WithPriority(3))
	runner.RegisterProcess(proc4, WithPriority(4), WithProcessName("bar"))

	go func() {
		defer close(errChan)

		for err := range runner.Run(nil, log.NewNilLogger()) {
			errChan <- err
		}
	}()

	// Whoops
	Eventually(errChan).Should(Receive(MatchError("bar returned a fatal error (error in start)")))

	// Processes stopped with reversed priority
	Eventually(stopChan).Should(Receive(Equal("proc4")))
	Eventually(stopChan).Should(Receive(Equal("proc3")))
	Eventually(stopChan).Should(Receive(Equal("proc2")))
	Eventually(stopChan).Should(Receive(Equal("proc1")))

	// Check additional errors on top
	Eventually(errChan).Should(Receive(MatchError("foo returned error from stop (error in stop)")))

	// Unblocked
	Eventually(errChan).Should(BeClosed())
}

func (s *RunnerSuite) TestInitializationError(t sweet.T) {
	var (
		runner   = NewProcessRunner(NewServiceContainer())
		initChan = make(chan string)
		stopChan = make(chan string)
		errChan  = make(chan error)
	)

	makeProcess := func(name string, initError error) Process {
		p := &mockProcess{}
		c := make(chan struct{})

		p.init = func(config Config) error {
			initChan <- name
			return initError
		}

		p.start = func() error {
			<-c
			return nil
		}

		p.stop = func() error {
			stopChan <- name
			close(c)
			return nil
		}

		return p
	}

	var (
		initError = errors.New("error in init")

		proc1 = makeProcess("proc1", nil)
		proc2 = makeProcess("proc2", nil)
		proc3 = makeProcess("proc3", nil)
		proc4 = makeProcess("proc4", initError)
		proc5 = makeProcess("proc5", nil)

		n1 string
		n2 string
	)

	runner.RegisterProcess(proc1, WithPriority(1))
	runner.RegisterProcess(proc2, WithPriority(2))
	runner.RegisterProcess(proc3, WithPriority(3))
	runner.RegisterProcess(proc4, WithPriority(3), WithProcessName("foo"))
	runner.RegisterProcess(proc5, WithPriority(3))

	go func() {
		defer close(errChan)

		for err := range runner.Run(nil, log.NewNilLogger()) {
			errChan <- err
		}
	}()

	// Initialization stops at error
	Eventually(initChan).Should(Receive(Equal("proc1")))
	Eventually(initChan).Should(Receive(Equal("proc2")))
	Eventually(initChan).Should(Receive(Equal("proc3")))
	Eventually(initChan).Should(Receive(Equal("proc4")))
	Consistently(initChan).ShouldNot(Receive())

	// Stop lower-priority processes which have already started.
	// Do not stop the proceses which have the same priority as
	// the process which just errored on init, as none of them
	// have been started.

	// NOTE: Eventually/Receive will skip values until the match
	// succeeds, so we need to peel off by reference so we can
	// check that the _exact next_ value is what we expect. We
	// don't want to skip over an erroneous proc3 on the channel.

	Eventually(stopChan).Should(Receive(&n1))
	Eventually(stopChan).Should(Receive(&n2))
	Expect(n1).To(Equal("proc2"))
	Expect(n2).To(Equal("proc1"))
	Consistently(stopChan).ShouldNot(Receive())

	// Check errors
	Eventually(errChan).Should(Receive(MatchError("failed to initialize foo (error in init)")))
	Eventually(errChan).Should(BeClosed())
}

//
// Mocks

type mockProcess struct {
	init  func(config Config) error
	start func() error
	stop  func() error
}

func (p *mockProcess) Init(config Config) error { return p.init(config) }
func (p *mockProcess) Start() error             { return p.start() }
func (p *mockProcess) Stop() error              { return p.stop() }
