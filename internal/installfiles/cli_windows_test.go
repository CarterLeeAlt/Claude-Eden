package installfiles

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIBoundariesAndSuccessfulTransaction(t *testing.T) {
	root := t.TempDir()
	target, receipt := filepath.Join(root, "staged"), filepath.Join(root, "receipt.json")
	app(t, target, "new")
	for _, args := range [][]string{{"-unknown"}, {"extra"}, {"-Operation", "prepare", "-Target", target}, {"-Operation", "uninstall", "-Target", target}, {"-Operation", "move", "-Target", target, "-Receipt", receipt}, {"-Operation", "unknown", "-Target", target}} {
		var output, stderr bytes.Buffer
		if code := CLI(context.Background(), args, &output, &stderr); code == 0 {
			t.Fatalf("accepted invalid CLI: %v", args)
		}
		if exists(receipt) {
			t.Fatal("invalid CLI published receipt")
		}
	}
	var output, stderr bytes.Buffer
	if code := CLI(context.Background(), []string{"-Operation", "prepare", "-Target", target, "-Receipt", receipt, "-KeepInPlace", "EdenData", "-KeepInPlace", "Projects"}, &output, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	r, e := readReceipt(receipt)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.KeepInPlace) != 2 {
		t.Fatal("repeated keep options lost", r.KeepInPlace)
	}
	if code := CLI(context.Background(), []string{"-Operation", "complete", "-Target", target, "-Receipt", receipt}, &output, &stderr); code != 0 {
		t.Fatal(code, stderr.String())
	}
	if exists(receipt) {
		t.Fatal("CLI failed to complete receipt")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := CLI(ctx, []string{"-Operation", "prepare", "-Target", target, "-Receipt", receipt}, &output, &stderr); code != 1 {
		t.Fatal("canceled CLI returned success", code)
	}
}

func TestPreparationRejectsOverlapAndKeepsExistingReceipt(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target")
	app(t, target, "new")
	receipt := filepath.Join(root, "receipt.json")
	for _, source := range []string{target, filepath.Join(target, "child"), root} {
		if e := Run(context.Background(), Options{Operation: "prepare", Source: source, Target: target, Receipt: receipt}); e == nil {
			t.Fatal("accepted overlapping source", source)
		}
	}
	writeFixture(t, receipt, "original receipt")
	if e := Run(context.Background(), Options{Operation: "prepare", Target: target, Receipt: receipt}); e == nil {
		t.Fatal("overwrote existing receipt")
	}
	requireText(t, receipt, "original receipt")
	requireText(t, filepath.Join(target, "program.txt"), "new")
}

func TestCleanupDoesNotDeleteDeduplicatedSourceWithoutDestination(t *testing.T) {
	root := t.TempDir()
	old, stage, receipt := filepath.Join(root, "old"), filepath.Join(root, "stage"), filepath.Join(root, "receipt.json")
	app(t, old, "old")
	app(t, stage, "new")
	writeFixture(t, filepath.Join(old, "shared.txt"), "same personal content")
	writeFixture(t, filepath.Join(stage, "shared.txt"), "same personal content")
	run(t, Options{Operation: "prepare", Source: old, Target: stage, Receipt: receipt, Backup: old + "-backup"})
	if e := os.Rename(old, old+"-backup"); e != nil {
		t.Fatal(e)
	}
	run(t, Options{Operation: "move", Source: old + "-backup", Target: stage, Receipt: receipt})
	if e := os.Remove(filepath.Join(stage, "shared.txt")); e != nil {
		t.Fatal(e)
	}
	if e := os.Rename(stage, old); e != nil {
		t.Fatal(e)
	}
	run(t, Options{Operation: "cleanup", Source: old + "-backup", Target: old, Receipt: receipt})
	if preservedTexts(t, old)["same personal content"] != 1 {
		t.Fatal("deduplicated personal source deleted after destination vanished")
	}
}
