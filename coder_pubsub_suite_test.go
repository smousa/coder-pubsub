package main_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestCoderPubsub(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CoderPubsub Suite")
}
