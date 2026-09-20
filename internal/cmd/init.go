package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"mini-me/internal/profile"
	"mini-me/internal/store"
)

var (
	flagInitName           string
	flagInitEmail          string
	flagInitPhone          string
	flagInitEmployer       string
	flagInitLinkedIn       string
	flagInitNonInteractive bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactive onboarding interview to set up your personal profile",
	Long:  `init asks a series of onboarding questions (or accepts flags) to populate deterministic identity fields, entities, and confirmed facts in mini-me.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		reader := bufio.NewReader(os.Stdin)

		hasFlags := flagInitName != "" || flagInitEmail != "" || flagInitPhone != "" || flagInitEmployer != "" || flagInitLinkedIn != ""
		skipPrompt := flagInitNonInteractive || hasFlags

		prompt := func(label, flagVal string) string {
			if flagVal != "" {
				return flagVal
			}
			if skipPrompt {
				return ""
			}
			fmt.Printf("%s: ", label)
			text, _ := reader.ReadString('\n')
			return strings.TrimSpace(text)
		}

		fmt.Println("=== mini-me Profile Setup Interview ===")
		name := prompt("Full Name", flagInitName)
		email := prompt("Email Address", flagInitEmail)
		phone := prompt("Phone Number", flagInitPhone)
		employer := prompt("Current Employer", flagInitEmployer)
		linkedin := prompt("LinkedIn URL", flagInitLinkedIn)

		// Set Fields
		if name != "" {
			if err := profile.ValidateFactContent("name", name); err != nil {
				return err
			}
			_ = st.SetField(ctx, "full_name", name)
		}
		if email != "" {
			if err := profile.ValidateFactContent("email", email); err != nil {
				return err
			}
			_ = st.SetField(ctx, "email", email)
		}
		if phone != "" {
			if err := profile.ValidateFactContent("phone", phone); err != nil {
				return err
			}
			_ = st.SetField(ctx, "phone", phone)
		}
		if employer != "" {
			_ = st.SetField(ctx, "current_employer", employer)
		}
		if linkedin != "" {
			_ = st.SetField(ctx, "linkedin_url", linkedin)
		}

		// Create Person & Org entities
		if name != "" {
			ent, _ := st.GetEntityByName(ctx, name)
			if ent == nil {
				entID, _ := st.CreateEntity(ctx, &store.Entity{
					Type: store.EntityPerson,
					Name: name,
				})
				if employer != "" {
					_ = profile.ValidateFactContent("works_at", employer)
					_, _ = st.CreateFact(ctx, &store.Fact{
						SubjectID:   entID,
						Predicate:   "works_at",
						Object:      employer,
						Status:      store.FactStatusConfirmed,
						Sensitivity: store.SensitivityNormal,
					})
				}
			}
		}

		if employer != "" {
			orgEnt, _ := st.GetEntityByName(ctx, employer)
			if orgEnt == nil {
				_, _ = st.CreateEntity(ctx, &store.Entity{
					Type: store.EntityOrg,
					Name: employer,
				})
			}
		}

		fmt.Println("\n[OK] Profile initialization complete! View your profile card anytime with `mini-me profile`.")
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&flagInitName, "name", "", "full name")
	initCmd.Flags().StringVar(&flagInitEmail, "email", "", "email address")
	initCmd.Flags().StringVar(&flagInitPhone, "phone", "", "phone number")
	initCmd.Flags().StringVar(&flagInitEmployer, "employer", "", "current employer")
	initCmd.Flags().StringVar(&flagInitLinkedIn, "linkedin", "", "linkedin url")
	initCmd.Flags().BoolVar(&flagInitNonInteractive, "non-interactive", false, "skip interactive prompts")
	RootCmd.AddCommand(initCmd)
}
