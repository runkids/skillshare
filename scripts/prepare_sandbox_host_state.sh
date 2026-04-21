#!/usr/bin/env bash
# Copy minimal Claude/Codex state into a sandbox home without bulk caches/marketplaces.
set -euo pipefail

HOST_HOME="${1:-$HOME}"
SANDBOX_HOME="${2:-/sandbox-home}"

copy_file() {
  local src="$1"
  local dst="$2"
  if [[ ! -f "$src" ]]; then
    return 0
  fi
  mkdir -p "$(dirname "$dst")"
  cp "$src" "$dst"
}

copy_dir() {
  local src="$1"
  local dst="$2"
  if [[ ! -d "$src" ]]; then
    return 0
  fi
  mkdir -p "$(dirname "$dst")"
  rm -rf "$dst"
  cp -R "$src" "$dst"
}

rewrite_home_prefix() {
  local file="$1"
  if [[ -f "$file" ]]; then
    HOST_HOME="$HOST_HOME" SANDBOX_HOME="$SANDBOX_HOME" perl -0pi -e 's/\Q$ENV{HOST_HOME}\E/$ENV{SANDBOX_HOME}/g' "$file"
  fi
}

extract_home_paths() {
  local file="$1"
  if [[ ! -f "$file" ]]; then
    return 0
  fi
  HOST_HOME="$HOST_HOME" perl -MJSON::PP -e '
    use strict;
    use warnings;

    my ($home, $path) = @ARGV;
    open my $fh, "<", $path or exit 0;
    local $/;
    my $json = <$fh>;
    close $fh;

    my $payload = eval { JSON::PP::decode_json($json) };
    exit 0 if !$payload;

    my %seen;

    sub emit_path {
      my ($candidate) = @_;
      return if !defined $candidate || $candidate eq q{};
      return if $seen{$candidate}++;
      print "$candidate\n";
    }

    sub maybe_emit {
      my ($command) = @_;
      return if !defined $command;
      $command =~ s/^\s+|\s+$//g;
      return if $command eq q{};

      if ($command =~ /^\s*(["\047])(\Q$home\E.+?)\1\s*$/) {
        emit_path($2);
        return;
      }
      if ($command =~ /^\s*(\Q$home\E\S*)\s*$/) {
        emit_path($1);
        return;
      }
      if ($command =~ /^\s*\S+\s+(["\047])(\Q$home\E.+?)\1(?:\s+.*)?$/) {
        emit_path($2);
        return;
      }
      if ($command =~ /^\s*\S+\s+(\Q$home\E\S*)(?:\s+.*)?$/) {
        emit_path($1);
      }
    }

    sub walk {
      my ($node) = @_;
      if (ref $node eq "HASH") {
        for my $key (keys %{$node}) {
          if ($key eq "command" && !ref $node->{$key}) {
            maybe_emit($node->{$key});
          }
          walk($node->{$key});
        }
        return;
      }
      if (ref $node eq "ARRAY") {
        walk($_) for @{$node};
      }
    }

    walk($payload);
  ' "$HOST_HOME" "$file" || true
}

copy_selected_codex_plugins() {
  local cfg="$1"
  if [[ ! -f "$cfg" ]]; then
    return 0
  fi
  local refs
  refs="$(sed -n 's/^\[plugins\."\([^"]\+\)"\]$/\1/p' "$cfg")"
  while IFS= read -r ref; do
    [[ -z "$ref" ]] && continue
    local name="${ref%@*}"
    local provider="${ref#*@}"
    [[ "$name" == "$provider" ]] && continue
    for candidate in "$HOST_HOME/.codex/plugins/cache/$provider/$name"/*; do
      [[ -d "$candidate" ]] || continue
      local rel="${candidate#$HOST_HOME/}"
      copy_dir "$candidate" "$SANDBOX_HOME/$rel"
    done
  done <<< "$refs"
}

mkdir -p "$SANDBOX_HOME"

copy_file "$HOST_HOME/.claude/settings.json" "$SANDBOX_HOME/.claude/settings.json"
copy_file "$HOST_HOME/.claude/hooks.json" "$SANDBOX_HOME/.claude/hooks.json"
copy_file "$HOST_HOME/.claude/plugins/installed_plugins.json" "$SANDBOX_HOME/.claude/plugins/installed_plugins.json"
copy_file "$HOST_HOME/.codex/config.toml" "$SANDBOX_HOME/.codex/config.toml"
copy_file "$HOST_HOME/.codex/hooks.json" "$SANDBOX_HOME/.codex/hooks.json"

while IFS= read -r path; do
  [[ -f "$path" ]] || continue
  rel="${path#$HOST_HOME/}"
  copy_file "$path" "$SANDBOX_HOME/$rel"
done < <(
  extract_home_paths "$HOST_HOME/.claude/settings.json"
  extract_home_paths "$HOST_HOME/.claude/hooks.json"
)

while IFS= read -r path; do
  [[ -d "$path" ]] || continue
  rel="${path#$HOST_HOME/}"
  copy_dir "$path" "$SANDBOX_HOME/$rel"
done < <(sed -n 's/.*"installPath"[[:space:]]*:[[:space:]]*"\([^"]\+\)".*/\1/p' "$HOST_HOME/.claude/plugins/installed_plugins.json" 2>/dev/null || true)

copy_selected_codex_plugins "$HOST_HOME/.codex/config.toml"

rewrite_home_prefix "$SANDBOX_HOME/.claude/settings.json"
rewrite_home_prefix "$SANDBOX_HOME/.claude/hooks.json"
rewrite_home_prefix "$SANDBOX_HOME/.claude/plugins/installed_plugins.json"
rewrite_home_prefix "$SANDBOX_HOME/.codex/config.toml"
rewrite_home_prefix "$SANDBOX_HOME/.codex/hooks.json"

echo "Prepared sandbox state in $SANDBOX_HOME"
