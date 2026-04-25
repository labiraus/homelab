#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/helheim-performance.sh <up|down>

up:
  Restore proxmox-node3 to the high-performance host policy used for GPU-heavy work.

down:
  Lower proxmox-node3 host power draw to reduce heat while helheim is idle.

Environment:
  HELHEIM_PERFORMANCE_HOST
    Override the SSH host alias. Defaults to proxmox-node3.
EOF
}

if [[ $# -ne 1 ]]; then
  usage
  exit 1
fi

mode="$1"
host="${HELHEIM_PERFORMANCE_HOST:-proxmox-node3}"

case "$mode" in
  up)
    target_profile="performance"
    target_governor="performance"
    target_no_turbo="0"
    ;;
  down)
    target_profile="low-power"
    target_governor="powersave"
    target_no_turbo="1"
    ;;
  *)
    usage
    exit 1
    ;;
esac

ssh "$host" env \
  MODE="$mode" \
  TARGET_PROFILE="$target_profile" \
  TARGET_GOVERNOR="$target_governor" \
  TARGET_NO_TURBO="$target_no_turbo" \
  bash -s <<'REMOTE'
set -euo pipefail

profile_path="/sys/firmware/acpi/platform_profile"
choices_path="/sys/firmware/acpi/platform_profile_choices"
no_turbo_path="/sys/devices/system/cpu/intel_pstate/no_turbo"

write_file() {
  local path="$1"
  local value="$2"

  if [[ -w "$path" ]]; then
    printf '%s' "$value" >"$path"
  else
    printf '%s' "$value" | sudo tee "$path" >/dev/null
  fi
}

read_file() {
  local path="$1"

  if [[ -r "$path" ]]; then
    cat "$path"
  fi
}

package_temp_c() {
  local path
  local raw

  for path in /sys/class/hwmon/hwmon*/temp*_label; do
    [[ -e "$path" ]] || continue
    if [[ "$(cat "$path")" == "Package id 0" ]]; then
      raw="$(cat "${path%_label}_input")"
      awk "BEGIN { printf \"%.1fC\", $raw / 1000 }"
      return 0
    fi
  done

  for path in /sys/class/thermal/thermal_zone*/type; do
    [[ -e "$path" ]] || continue
    if [[ "$(cat "$path")" == "x86_pkg_temp" ]]; then
      raw="$(cat "${path%/type}/temp")"
      awk "BEGIN { printf \"%.1fC\", $raw / 1000 }"
      return 0
    fi
  done

  printf 'unknown'
}

cpu_governors() {
  local governors
  governors="$(grep . /sys/devices/system/cpu/cpu*/cpufreq/scaling_governor 2>/dev/null | cut -d: -f2 | sort -u | paste -sd, -)"
  printf '%s' "${governors:-unknown}"
}

cpu_freq_summary() {
  awk '
    BEGIN { min=-1; max=0; count=0; sum=0 }
    {
      val = $1 + 0
      if (min < 0 || val < min) min = val
      if (val > max) max = val
      sum += val
      count++
    }
    END {
      if (count == 0) {
        print "unknown"
        exit
      }
      printf "min=%.2fGHz avg=%.2fGHz max=%.2fGHz",
        min / 1000000, (sum / count) / 1000000, max / 1000000
    }
  ' /sys/devices/system/cpu/cpu*/cpufreq/scaling_cur_freq 2>/dev/null
}

print_state() {
  printf 'host=%s\n' "$(hostname)"
  printf 'platform_profile=%s\n' "$(read_file "$profile_path" || printf 'unavailable')"
  printf 'governor=%s\n' "$(cpu_governors)"
  if [[ -e "$no_turbo_path" ]]; then
    printf 'intel_pstate.no_turbo=%s\n' "$(read_file "$no_turbo_path")"
  else
    printf 'intel_pstate.no_turbo=unavailable\n'
  fi
  printf 'cpu_freq=%s\n' "$(cpu_freq_summary)"
  printf 'package_temp=%s\n' "$(package_temp_c)"
}

choose_profile() {
  local requested="$1"
  local choices
  local candidate

  choices="$(read_file "$choices_path" || true)"
  if [[ -z "$choices" ]]; then
    printf '%s' "$requested"
    return 0
  fi

  case "$requested" in
    low-power)
      for candidate in low-power balanced balanced-performance performance; do
        if grep -qw "$candidate" <<<"$choices"; then
          printf '%s' "$candidate"
          return 0
        fi
      done
      ;;
    performance)
      for candidate in performance balanced-performance balanced; do
        if grep -qw "$candidate" <<<"$choices"; then
          printf '%s' "$candidate"
          return 0
        fi
      done
      ;;
    *)
      if grep -qw "$requested" <<<"$choices"; then
        printf '%s' "$requested"
        return 0
      fi
      ;;
  esac

  for candidate in $choices; do
    if [[ "$candidate" != "quiet" ]]; then
      printf '%s' "$candidate"
      return 0
    fi
  done

  printf '%s' "$requested"
}

apply_mode() {
  local requested_profile
  local effective_profile
  local governor_path

  requested_profile="$(choose_profile "$TARGET_PROFILE")"
  effective_profile="$requested_profile"

  if [[ -e "$profile_path" ]]; then
    write_file "$profile_path" "$effective_profile"
  fi

  for governor_path in /sys/devices/system/cpu/cpufreq/policy*/scaling_governor; do
    [[ -e "$governor_path" ]] || continue
    write_file "$governor_path" "$TARGET_GOVERNOR"
  done

  if [[ -e "$no_turbo_path" ]]; then
    write_file "$no_turbo_path" "$TARGET_NO_TURBO"
  fi
}

printf 'before:\n'
print_state
printf '\napplying mode=%s\n\n' "$MODE"
apply_mode
sleep 2
printf 'after:\n'
print_state
REMOTE
