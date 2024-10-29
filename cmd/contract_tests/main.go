package main

import (
	"fmt"
	"log"

	"github.com/snikch/goodman/hooks"
	"github.com/snikch/goodman/transaction"
)

const (
	endpointVersion     = "/version > GET"
	endpointNodeVersion = "/node_version > GET"
)

type HookManager struct {
	hooks   *hooks.Hooks
	server  *hooks.Server
	logger  *log.Logger
}

func NewHookManager() *HookManager {
	h := hooks.NewHooks()
	return &HookManager{
		hooks:   h,
		server:  hooks.NewServer(hooks.NewHooksRunner(h)),
		logger:  log.New(log.Writer(), "[Dredd Hooks] ", log.LstdFlags),
	}
}

func (hm *HookManager) registerHooks() {
	hm.registerGlobalHooks()
	hm.registerEndpointSpecificHooks()
}

func (hm *HookManager) registerGlobalHooks() {
	// BeforeAll hooks
	hm.hooks.BeforeAll(func(t []*transaction.Transaction) {
		hm.logger.Println("Sleep 5 seconds before all modification")
	})

	// BeforeEach hooks
	hm.hooks.BeforeEach(func(t *transaction.Transaction) {
		hm.logger.Println("before each modification")
	})

	// BeforeEachValidation hooks
	hm.hooks.BeforeEachValidation(func(t *transaction.Transaction) {
		hm.logger.Println("before each validation modification")
	})

	// AfterEach hooks
	hm.hooks.AfterEach(func(t *transaction.Transaction) {
		hm.logger.Println("after each modification")
	})

	// AfterAll hooks
	hm.hooks.AfterAll(func(t []*transaction.Transaction) {
		hm.logger.Println("after all modification")
	})
}

func (hm *HookManager) registerEndpointSpecificHooks() {
	// Version endpoint hooks
	hm.hooks.Before(endpointVersion, func(t *transaction.Transaction) {
		hm.logger.Println("before version TEST")
	})

	// NodeVersion endpoint hooks
	hm.registerNodeVersionHooks()
}

func (hm *HookManager) registerNodeVersionHooks() {
	// Before hook
	hm.hooks.Before(endpointNodeVersion, func(t *transaction.Transaction) {
		hm.logger.Println("before node_version TEST")
	})

	// BeforeValidation hook
	hm.hooks.BeforeValidation(endpointNodeVersion, func(t *transaction.Transaction) {
		hm.logger.Println("before validation node_version TEST")
	})

	// After hook
	hm.hooks.After(endpointNodeVersion, func(t *transaction.Transaction) {
		hm.logger.Println("after node_version TEST")
	})
}

func (hm *HookManager) start() error {
	defer hm.server.Listener.Close()
	
	hm.logger.Println("Starting Dredd hooks server...")
	if err := hm.server.Serve(); err != nil {
		return fmt.Errorf("failed to start hooks server: %w", err)
	}
	return nil
}

func main() {
	hookManager := NewHookManager()
	hookManager.registerHooks()

	if err := hookManager.start(); err != nil {
		log.Fatalf("Error running hooks server: %v", err)
	}
}
