package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Player struct {
	mu      sync.Mutex
	playing bool
	token   uint64
	active  *playbackProcess
}

type playbackProcess struct {
	cmd     *exec.Cmd
	cleanup func()
}

type playbackCandidate struct {
	name string
	args func(path string) []string
}

func NewPlayer() *Player {
	return &Player{}
}

func (p *Player) Play(filename, mimeType string, data []byte) error {
	if err := p.Stop(); err != nil {
		return err
	}

	path, cleanup, err := writeTempAudioFile(filename, mimeType, data)
	if err != nil {
		return err
	}
	cmd, err := commandFor(path)
	if err != nil {
		cleanup()
		return err
	}
	if err := cmd.Start(); err != nil {
		cleanup()
		return fmt.Errorf("start voice note playback: %w", err)
	}

	proc := &playbackProcess{cmd: cmd, cleanup: cleanup}
	p.mu.Lock()
	p.token++
	token := p.token
	p.active = proc
	p.playing = true
	p.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		cleanup()
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.token == token {
			p.active = nil
			p.playing = false
		}
	}()

	return nil
}

func (p *Player) Stop() error {
	p.mu.Lock()
	proc := p.active
	p.token++
	p.active = nil
	p.playing = false
	p.mu.Unlock()
	if proc != nil {
		proc.stop()
	}
	return nil
}

func (p *Player) Close() error {
	return p.Stop()
}

func (p *Player) IsPlaying() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing
}

func (p *playbackProcess) stop() {
	if p == nil {
		return
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	if p.cleanup != nil {
		p.cleanup()
	}
}

func writeTempAudioFile(filename, mimeType string, data []byte) (string, func(), error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = extFromMIME(mimeType)
	}
	if ext == "" {
		ext = ".bin"
	}

	tmp, err := os.CreateTemp("", "pando-voice-*"+ext)
	if err != nil {
		return "", nil, fmt.Errorf("create temp audio file: %w", err)
	}
	cleanup := onceFunc(func() { _ = os.Remove(tmp.Name()) })
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return "", nil, fmt.Errorf("write temp audio file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("close temp audio file: %w", err)
	}
	return tmp.Name(), cleanup, nil
}

func commandFor(path string) (*exec.Cmd, error) {
	ext := strings.ToLower(filepath.Ext(path))
	tried := make([]string, 0, 6)
	for _, candidate := range playbackCandidates(runtime.GOOS, ext) {
		if _, err := exec.LookPath(candidate.name); err != nil {
			tried = append(tried, candidate.name)
			continue
		}
		return exec.Command(candidate.name, candidate.args(path)...), nil
	}
	return nil, fmt.Errorf("no supported audio player found; install one of: %s", strings.Join(uniqueStrings(tried), ", "))
}

func extFromMIME(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0]))
	switch mimeType {
	case "audio/wav", "audio/x-wav", "audio/wave":
		return ".wav"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "audio/mp4", "audio/x-m4a":
		return ".m4a"
	case "audio/aac", "audio/aacp":
		return ".aac"
	case "audio/ogg", "application/ogg":
		return ".ogg"
	case "audio/opus":
		return ".opus"
	case "audio/webm":
		return ".webm"
	default:
		return ""
	}
}

func playbackCandidates(goos, ext string) []playbackCandidate {
	candidates := []playbackCandidate{
		{name: "mpv", args: func(path string) []string { return []string{"--no-video", "--really-quiet", path} }},
		{name: "ffplay", args: func(path string) []string { return []string{"-nodisp", "-autoexit", "-loglevel", "error", path} }},
		{name: "cvlc", args: func(path string) []string { return []string{"--intf", "dummy", "--play-and-exit", "--quiet", path} }},
	}
	if goos == "darwin" {
		candidates = append(candidates, playbackCandidate{name: "afplay", args: func(path string) []string { return []string{path} }})
	}
	if ext == ".wav" {
		candidates = append(candidates,
			playbackCandidate{name: "pw-play", args: func(path string) []string { return []string{path} }},
			playbackCandidate{name: "paplay", args: func(path string) []string { return []string{path} }},
			playbackCandidate{name: "aplay", args: func(path string) []string { return []string{path} }},
		)
	}
	return candidates
}

func onceFunc(fns ...func()) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			for _, fn := range fns {
				if fn != nil {
					fn()
				}
			}
		})
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}
