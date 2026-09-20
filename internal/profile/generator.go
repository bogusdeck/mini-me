package profile

import (
	"context"
	"fmt"
	"strings"

	"mini-me/internal/store"
)

type Generator struct {
	store *store.Store
}

func NewGenerator(st *store.Store) *Generator {
	return &Generator{store: st}
}

// GenerateProfileCard synthesizes a deterministic markdown profile card (<= 1500 tokens).
func (g *Generator) GenerateProfileCard(ctx context.Context) (string, error) {
	var sb strings.Builder

	sb.WriteString("# Personal Profile Card\n\n")

	// 1. Identity Fields
	fields, err := g.store.ListFields(ctx)
	if err != nil {
		return "", fmt.Errorf("failed fetching profile fields: %w", err)
	}

	sb.WriteString("## Identity & Contact\n")
	if len(fields) == 0 {
		sb.WriteString("_No profile identity fields set. Run `mini-me init` to set up profile._\n\n")
	} else {
		for _, f := range fields {
			keyFormatted := strings.Title(strings.ReplaceAll(f.Key, "_", " "))
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", keyFormatted, f.Value))
		}
		sb.WriteString("\n")
	}

	// 2. Entities (Projects & Orgs)
	entities, err := g.store.ListEntities(ctx)
	if err != nil {
		return "", fmt.Errorf("failed fetching entities: %w", err)
	}

	var projects, orgs, people []*store.Entity
	for _, e := range entities {
		switch e.Type {
		case store.EntityProject:
			projects = append(projects, e)
		case store.EntityOrg:
			orgs = append(orgs, e)
		case store.EntityPerson:
			people = append(people, e)
		}
	}

	sb.WriteString("## Projects & Organizations\n")
	if len(projects) == 0 && len(orgs) == 0 {
		sb.WriteString("_No projects or organizations recorded._\n\n")
	} else {
		if len(orgs) > 0 {
			sb.WriteString("### Organizations\n")
			for _, o := range orgs {
				notesStr := ""
				if o.Notes != "" {
					notesStr = fmt.Sprintf(" - %s", o.Notes)
				}
				sb.WriteString(fmt.Sprintf("- **%s**%s\n", o.Name, notesStr))
			}
		}
		if len(projects) > 0 {
			sb.WriteString("### Projects\n")
			for _, p := range projects {
				notesStr := ""
				if p.Notes != "" {
					notesStr = fmt.Sprintf(" - %s", p.Notes)
				}
				sb.WriteString(fmt.Sprintf("- **%s**%s\n", p.Name, notesStr))
			}
		}
		sb.WriteString("\n")
	}

	// 3. People & Network
	if len(people) > 0 {
		sb.WriteString("## People & Network\n")
		for _, p := range people {
			notesStr := ""
			if p.Notes != "" {
				notesStr = fmt.Sprintf(" - %s", p.Notes)
			}
			sb.WriteString(fmt.Sprintf("- **%s**%s\n", p.Name, notesStr))
		}
		sb.WriteString("\n")
	}

	// 4. Confirmed Facts & Knowledge
	confirmedFacts, err := g.store.ListFactsByStatus(ctx, store.FactStatusConfirmed)
	if err != nil {
		return "", fmt.Errorf("failed fetching confirmed facts: %w", err)
	}

	sb.WriteString("## Confirmed Facts & Knowledge\n")
	if len(confirmedFacts) == 0 {
		sb.WriteString("_No confirmed facts in knowledge graph._\n\n")
	} else {
		// Group facts by subject
		entMap := make(map[int64]*store.Entity)
		for _, e := range entities {
			entMap[e.ID] = e
		}

		for _, f := range confirmedFacts {
			subjName := fmt.Sprintf("Subject #%d", f.SubjectID)
			if ent, ok := entMap[f.SubjectID]; ok {
				subjName = ent.Name
			}
			sensTag := ""
			if f.Sensitivity == store.SensitivityNeverInfer {
				sensTag = " [never_infer]"
			}
			sb.WriteString(fmt.Sprintf("- **%s** %s %s%s\n", subjName, f.Predicate, f.Object, sensTag))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}
