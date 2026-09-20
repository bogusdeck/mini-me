package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"mini-me/internal/profile"
	"mini-me/internal/store"
)

var (
	flagFactSensitivity string
	flagFactStatus      string
	flagFactListStatus  string
)

var factCmd = &cobra.Command{
	Use:   "fact",
	Short: "Manage knowledge graph facts (add, list, confirm, reject, forget)",
}

var factAddCmd = &cobra.Command{
	Use:   "add <subject> <predicate> <object>",
	Short: "Add a fact to the knowledge graph",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		subjName, predicate, object := args[0], args[1], args[2]

		// Safety check against SSN & Credit Cards
		if err := profile.ValidateFactContent(predicate, object); err != nil {
			return err
		}

		sensitivity := store.FactSensitivity(strings.ToLower(flagFactSensitivity))
		if sensitivity == "" {
			sensitivity = store.SensitivityNormal
		}

		// Enforce manual check for never_infer
		if err := profile.ValidateFactSensitivity(sensitivity, true); err != nil {
			return err
		}

		status := store.FactStatus(strings.ToLower(flagFactStatus))
		if status == "" {
			status = store.FactStatusConfirmed
		}

		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		// Retrieve or create subject entity
		ent, err := st.GetEntityByName(ctx, subjName)
		if err != nil {
			return fmt.Errorf("failed fetching entity: %w", err)
		}
		var subjID int64
		if ent == nil {
			subjID, err = st.CreateEntity(ctx, &store.Entity{
				Type: store.EntityPerson,
				Name: subjName,
			})
			if err != nil {
				return fmt.Errorf("failed creating entity: %w", err)
			}
		} else {
			subjID = ent.ID
		}

		fact := &store.Fact{
			SubjectID:   subjID,
			Predicate:   predicate,
			Object:      object,
			Status:      status,
			Sensitivity: sensitivity,
		}

		factID, err := st.CreateFact(ctx, fact)
		if err != nil {
			return fmt.Errorf("failed creating fact: %w", err)
		}

		fmt.Printf("Fact #%d added: %s %s %s [%s, %s]\n", factID, subjName, predicate, object, status, sensitivity)
		return nil
	},
}

var factListCmd = &cobra.Command{
	Use:   "list",
	Short: "List facts in the knowledge graph",
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

		status := store.FactStatusConfirmed
		if flagFactListStatus != "" {
			status = store.FactStatus(strings.ToLower(flagFactListStatus))
		}

		facts, err := st.ListFactsByStatus(ctx, status)
		if err != nil {
			return fmt.Errorf("failed listing facts: %w", err)
		}

		if len(facts) == 0 {
			fmt.Printf("No facts found with status %q.\n", status)
			return nil
		}

		entities, _ := st.ListEntities(ctx)
		entMap := make(map[int64]string)
		for _, e := range entities {
			entMap[e.ID] = e.Name
		}

		fmt.Printf("Facts (status: %s):\n", status)
		for _, f := range facts {
			subjName := entMap[f.SubjectID]
			if subjName == "" {
				subjName = fmt.Sprintf("Subject #%d", f.SubjectID)
			}
			fmt.Printf("  #%d: %s %s %s [%s]\n", f.ID, subjName, f.Predicate, f.Object, f.Sensitivity)
		}

		return nil
	},
}

var factConfirmCmd = &cobra.Command{
	Use:   "confirm <fact_id>",
	Short: "Confirm a proposed fact",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid fact ID: %w", err)
		}

		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		if err := st.UpdateFactStatus(ctx, id, store.FactStatusConfirmed); err != nil {
			return fmt.Errorf("failed confirming fact #%d: %w", id, err)
		}

		fmt.Printf("Fact #%d updated to status 'confirmed'.\n", id)
		return nil
	},
}

var factRejectCmd = &cobra.Command{
	Use:   "reject <fact_id>",
	Short: "Reject a proposed fact",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid fact ID: %w", err)
		}

		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		if err := st.UpdateFactStatus(ctx, id, store.FactStatusRejected); err != nil {
			return fmt.Errorf("failed rejecting fact #%d: %w", id, err)
		}

		fmt.Printf("Fact #%d updated to status 'rejected'.\n", id)
		return nil
	},
}

var factForgetCmd = &cobra.Command{
	Use:   "forget <fact_id>",
	Short: "Delete a fact from the knowledge graph",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid fact ID: %w", err)
		}

		dbPath, err := GetDBPath()
		if err != nil {
			return fmt.Errorf("failed resolving db path: %w", err)
		}

		st, err := store.Open(ctx, dbPath)
		if err != nil {
			return fmt.Errorf("failed opening database: %w", err)
		}
		defer st.Close()

		_, err = st.DB().ExecContext(ctx, "DELETE FROM fact WHERE id = ?", id)
		if err != nil {
			return fmt.Errorf("failed deleting fact #%d: %w", id, err)
		}

		fmt.Printf("Fact #%d deleted.\n", id)
		return nil
	},
}

func init() {
	factAddCmd.Flags().StringVar(&flagFactSensitivity, "sensitivity", "normal", "sensitivity level (normal, personal, never_infer)")
	factAddCmd.Flags().StringVar(&flagFactStatus, "status", "confirmed", "fact status (confirmed, proposed)")
	factListCmd.Flags().StringVar(&flagFactListStatus, "status", "confirmed", "filter by status (confirmed, proposed, rejected)")

	factCmd.AddCommand(factAddCmd, factListCmd, factConfirmCmd, factRejectCmd, factForgetCmd)
	RootCmd.AddCommand(factCmd)
}
