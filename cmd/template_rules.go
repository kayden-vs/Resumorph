package cmd

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type markerInfo struct {
	Section          string
	InResumeItem     bool
	InProjectHeading bool
}

var sectionRe = regexp.MustCompile(`\\section\{([^}]*)\}`)

func parseTemplateMarkers(templatePath string) (map[string]markerInfo, error) {
	content, err := os.ReadFile(templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("template file not found: %w", errors.New(templatePath))
		}
		return nil, fmt.Errorf("read template file: %w", err)
	}

	markers := make(map[string]markerInfo)
	currentSection := "header"
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		if match := sectionRe.FindStringSubmatch(line); len(match) == 2 {
			currentSection = strings.TrimSpace(match[1])
		}

		matches := markerRe.FindAllStringSubmatch(line, -1)
		if len(matches) == 0 {
			continue
		}

		inResumeItem := strings.Contains(line, `\resumeItem{`)
		inProjectHeading := strings.Contains(line, `\resumeProjectHeading`)

		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			key := match[1]
			info := markers[key]
			if info.Section == "" {
				info.Section = currentSection
			}
			if inResumeItem {
				info.InResumeItem = true
			}
			if inProjectHeading {
				info.InProjectHeading = true
			}
			markers[key] = info
		}
	}

	return markers, nil
}

func isHeaderSection(section string) bool {
	return strings.EqualFold(strings.TrimSpace(section), "header")
}

func isEducationSection(section string) bool {
	return strings.Contains(strings.ToLower(section), "education")
}

func isAchievementsSection(section string) bool {
	return strings.Contains(strings.ToLower(section), "achievement")
}

func isProjectsSection(section string) bool {
	return strings.Contains(strings.ToLower(section), "project")
}

func isSkillsSection(section string) bool {
	lower := strings.ToLower(strings.TrimSpace(section))
	if strings.Contains(lower, "technical skills") {
		return true
	}
	return strings.Contains(lower, "skill")
}
