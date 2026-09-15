package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// A shift written in UTC comes back from the API in the schedule's timezone.
// Before keepEquivalentScheduleTimes, apply failed with "Provider produced
// inconsistent result after apply" for .shifts[0].start_at.
func TestKeepEquivalentScheduleTimesPreservesConfiguredOffset(t *testing.T) {
	prior := scheduleModelFromAPI(map[string]any{
		"timezone": "UTC",
		"shifts":   []any{map[string]any{"user_id": "u1", "start_at": "2026-09-14T00:00:00Z", "end_at": "2026-09-21T00:00:00Z"}},
		"rotation": map[string]any{"start_at": "2026-09-14T06:00:00Z", "participant_ids": []any{"u1"}},
	})
	model := scheduleModelFromAPI(map[string]any{
		"timezone": "Europe/Moscow",
		"shifts":   []any{map[string]any{"user_id": "u1", "start_at": "2026-09-14T00:00:00+00:00", "end_at": "2026-09-21T00:00:00+00:00"}},
		"rotation": map[string]any{"start_at": "2026-09-14T06:00:00+00:00", "participant_ids": []any{"u1"}},
	})
	keepEquivalentScheduleTimes(&model, prior)

	shift := model.Shifts.Elements()[0].(types.Object).Attributes()
	assertEqual(t, "shift start_at", "2026-09-14T00:00:00Z", shift["start_at"].(types.String).ValueString())
	assertEqual(t, "shift end_at", "2026-09-21T00:00:00Z", shift["end_at"].(types.String).ValueString())
	rotation := model.Rotation.Attributes()
	assertEqual(t, "rotation start_at", "2026-09-14T06:00:00Z", rotation["start_at"].(types.String).ValueString())
}

func TestKeepEquivalentScheduleTimesReportsRealDrift(t *testing.T) {
	prior := scheduleModelFromAPI(map[string]any{
		"timezone": "UTC",
		"shifts":   []any{map[string]any{"user_id": "u1", "start_at": "2026-09-14T00:00:00Z", "end_at": "2026-09-21T00:00:00Z"}},
	})
	model := scheduleModelFromAPI(map[string]any{
		"timezone": "Europe/Moscow",
		"shifts":   []any{map[string]any{"user_id": "u1", "start_at": "2026-09-15T00:00:00+00:00", "end_at": "2026-09-21T00:00:00+00:00"}},
	})
	keepEquivalentScheduleTimes(&model, prior)

	shift := model.Shifts.Elements()[0].(types.Object).Attributes()
	assertEqual(t, "moved start_at", "2026-09-15T03:00:00+03:00", shift["start_at"].(types.String).ValueString())
	assertEqual(t, "unchanged end_at", "2026-09-21T00:00:00Z", shift["end_at"].(types.String).ValueString())
}

// A schedule gaining a shift keeps the known ones and takes the new one as returned.
func TestKeepEquivalentScheduleTimesToleratesAddedShift(t *testing.T) {
	prior := scheduleModelFromAPI(map[string]any{
		"timezone": "UTC",
		"shifts":   []any{map[string]any{"user_id": "u1", "start_at": "2026-09-14T00:00:00Z", "end_at": "2026-09-21T00:00:00Z"}},
	})
	model := scheduleModelFromAPI(map[string]any{
		"timezone": "Europe/Moscow",
		"shifts": []any{
			map[string]any{"user_id": "u1", "start_at": "2026-09-14T00:00:00+00:00", "end_at": "2026-09-21T00:00:00+00:00"},
			map[string]any{"user_id": "u2", "start_at": "2026-09-21T00:00:00+00:00", "end_at": "2026-09-28T00:00:00+00:00"},
		},
	})
	keepEquivalentScheduleTimes(&model, prior)

	shifts := model.Shifts.Elements()
	assertEqual(t, "shift count", "2", string(rune('0'+len(shifts))))
	second := shifts[1].(types.Object).Attributes()
	assertEqual(t, "new shift start_at", "2026-09-21T03:00:00+03:00", second["start_at"].(types.String).ValueString())
}

func TestKeepEquivalentOverrideTimes(t *testing.T) {
	prior := scheduleOverrideModel{
		StartAt: types.StringValue("2026-09-16T00:00:00Z"),
		Until:   types.StringValue("2026-09-16T12:00:00Z"),
	}
	model := scheduleOverrideModelFromAPI(
		map[string]any{"start_at": "2026-09-16T00:00:00+00:00", "until": "2026-09-16T13:00:00+00:00"},
		map[string]any{"timezone": "Europe/Moscow"}, "sch-1")
	keepEquivalentOverrideTimes(&model, prior)

	assertEqual(t, "start_at", "2026-09-16T00:00:00Z", model.StartAt.ValueString())
	assertEqual(t, "moved until", "2026-09-16T16:00:00+03:00", model.Until.ValueString())
}
