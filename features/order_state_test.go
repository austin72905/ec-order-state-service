package features

import (
	"ec-order-state-service/features/step_definitions"
	"testing"

	"github.com/cucumber/godog"
)

func TestFeatures(t *testing.T) {
	suite := godog.TestSuite{
		ScenarioInitializer: step_definitions.InitializeScenario,
		Options: &godog.Options{
			Format:   "pretty", // 或 "junit", "json", "progress"
			Paths:    []string{"."}, // 當前目錄（features 目錄）
			TestingT: t,
		},
	}

	if suite.Run() != 0 {
		t.Fatal("BDD 測試失敗")
	}
}

