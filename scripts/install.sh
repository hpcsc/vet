#!/bin/sh

# vet — first-time installer.
#
# Installs the vet binary from the hpcsc/vet GitHub releases into a directory
# (default ~/.local/bin). It asks which release channel to use (release or
# prerelease), lists the available versions in that channel for selection,
# then asks where to put the binary.
#
# Requirements: curl, jq, tar, gzip, and sha256sum (or shasum on macOS).
#
# Environment (also used by the end-to-end tests):
#   GITHUB_TOKEN    the token used to fetch a private repository, optional
#   GH_TOKEN        a fallback name for the same token
#   GITHUB_API_URL  the base of the GitHub API, default https://api.github.com
#   VET_INSTALL_DIR the default install directory, default ~/.local/bin
set -eu

repo="hpcsc/vet"
binary_name="vet"
archive_name_base="vet"
checksums_name="checksums.txt"
api="${GITHUB_API_URL:-https://api.github.com}"
token="${GITHUB_TOKEN:-${GH_TOKEN:-}}"
install_dir="${VET_INSTALL_DIR:-${HOME}/.local/bin}"

die() {
	printf 'install: %s\n' "$*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "missing dependency: $1"
}

platform() {
	os=$(uname -s)
	case "$os" in
	Linux) goos=linux ;;
	Darwin) goos=darwin ;;
	*) die "unsupported operating system: $os" ;;
	esac

	machine=$(uname -m)
	case "$machine" in
	x86_64 | amd64) goarch=amd64 ;;
	aarch64 | arm64) goarch=arm64 ;;
	*) die "unsupported architecture: $machine" ;;
	esac

	printf '%s-%s\n' "$goos" "$goarch"
}

auth_args=
if [ -n "$token" ]; then
	auth_args="-H Authorization: Bearer $token"
fi

# api_releases prints the JSON body of the releases page.
api_releases() {
	curl -fsSL --retry 3 \
		-H 'Accept: application/vnd.github+json' \
		-H 'X-GitHub-Api-Version: 2022-11-28' \
		-H 'user-agent: vet-install' \
		$auth_args \
		"$api/repos/$repo/releases?per_page=100" 2>/dev/null
}

# download_asset fetches a release asset through its API URL.
download_asset() {
	url=$1
	curl -fsSL --retry 3 \
		-H 'Accept: application/octet-stream' \
		-H 'user-agent: vet-install' \
		$auth_args \
		"$url" 2>/dev/null
}

checksummer=
if command -v sha256sum >/dev/null 2>&1; then
	checksummer=sha256sum
elif command -v shasum >/dev/null 2>&1; then
	checksummer='shasum -a 256'
else
	die 'missing dependency: sha256sum (or shasum)'
fi

usage() {
	cat <<'EOF'
Usage: install.sh [OPTIONS]

  --dir PATH       install directory, default ~/.local/bin
  --channel CH     release or prerelease; asked interactively when omitted
  -h, --help       print this help
EOF
}

channel=
dir_flag=
while [ "$#" -gt 0 ]; do
	case "$1" in
	--dir)
		shift
		[ "$#" -gt 0 ] || die '--dir needs a value'
		install_dir=$1
		dir_flag=yes
		;;
	--channel)
		shift
		[ "$#" -gt 0 ] || die '--channel needs a value'
		channel=$1
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		die "unknown argument: $1"
		;;
	esac
	shift
done

for tool in curl jq tar gzip; do
	need "$tool"
done

platform_name=$(platform)
archive_name="$archive_name_base-$platform_name.tar.gz"

if [ -n "$channel" ]; then
	case "$channel" in
	release | prerelease) ;;
	*) die "unknown channel: $channel (release or prerelease)" ;;
	esac
else
	printf 'Which release channel do you want to install from?\n'
	printf '  1) release    the latest stable release\n'
	printf '  2) prerelease the latest pre-release\n'
	printf 'Choose [1]: '
	read -r choice || :
	case "${choice:-1}" in
	1 | '') channel=release ;;
	2) channel=prerelease ;;
	*) die "invalid choice: $choice" ;;
	esac
fi

releases_json=$(api_releases) || die 'could not fetch the releases of hpcsc/vet'

if [ "$channel" = prerelease ]; then
	matches=$(printf '%s\n' "$releases_json" |
		jq -r '[.[] | select(.prerelease == true) | .tag_name] | reverse | .[]')
else
	matches=$(printf '%s\n' "$releases_json" |
		jq -r '[.[] | select(.prerelease == false) | .tag_name] | reverse | .')
fi
[ -n "$matches" ] || die "there is no $channel release of vet yet"

# List the versions in the chosen channel for selection.
printf '\nAvailable vet %s releases:\n' "$channel"
n=0
printf '%s\n' "$matches" | while IFS= read -r tag; do
	n=$((n + 1))
	printf '  %2d) %s\n' "$n" "$tag"
done
version_count=$(printf '%s\n' "$matches" | sed -n '$=')

if [ "$version_count" -eq 1 ]; then
	tag=$matches
else
	printf 'Which version? [1]: '
	read -r pick || :
	case "$pick" in
	'' | *[!0-9]*) pick=1 ;;
	esac
	if [ "$pick" -lt 1 ] || [ "$pick" -gt "$version_count" ]; then
		die "invalid version: $pick"
	fi
	tag=$(printf '%s\n' "$matches" | sed -n "${pick}p")
fi

if [ -z "$dir_flag" ]; then
	printf 'Install vet to [%s]: ' "$install_dir"
	read -r custom || :
	if [ -n "$custom" ]; then
		install_dir=$custom
	fi
fi

release_json=$(printf '%s\n' "$releases_json" | jq -c --arg tag "$tag" '.[] | select(.tag_name == $tag)')
[ -n "$release_json" ] || die "could not find release $tag"

archive_url=$(printf '%s\n' "$release_json" |
	jq -r --arg name "$archive_name" '.assets[] | select(.name == $name) | .url')
checksums_url=$(printf '%s\n' "$release_json" |
	jq -r --arg name "$checksums_name" '.assets[] | select(.name == $name) | .url')
[ -n "$archive_url" ] && [ "$archive_url" != null ] || die "release $tag has no $archive_name"
[ -n "$checksums_url" ] && [ "$checksums_url" != null ] || die "release $tag has no $checksums_name"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

download_asset "$archive_url" >"$tmp/$archive_name"
download_asset "$checksums_url" >"$tmp/$checksums_name"

expected=$(awk -v name="$archive_name" '$2 == name { print $1 }' "$tmp/$checksums_name")
[ -n "$expected" ] || die "$checksums_name has no line for $archive_name"

actual=$($checksummer "$tmp/$archive_name" | awk '{ print $1 }')
[ "$expected" = "$actual" ] || die "checksum mismatch for $archive_name: got $actual, want $expected"

mkdir -p "$tmp/root"
tar -xzf "$tmp/$archive_name" -C "$tmp/root"
binary=$(find "$tmp/root" -type f -name "$binary_name" | head -n 1)
[ -n "$binary" ] || die "$archive_name does not contain a $binary_name binary"

mkdir -p "$install_dir"
install -m 0755 "$binary" "$install_dir/$binary_name"

printf '\nvet %s installed to %s/%s\n' "$tag" "$install_dir" "$binary_name"
case ":$PATH:" in
*":$install_dir:"*) ;;
*)
	printf 'Add %s to your PATH, for example:\n  export PATH=%s:$PATH\n' "$install_dir" "$install_dir"
	;;
esac
