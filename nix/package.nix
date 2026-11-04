{
  lib,
  buildGoModule,
  gitMinimal,
  runtimeShell,
  versionCheckHook,
  writableTmpDirAsHomeHook,
  # Release Please keeps this manifest at the latest released version.
  version ? (lib.importJSON ../.github/release-please-manifest.json).".",
}:

let
  inherit (lib) fileset;
  goModVersion = lib.removePrefix "go " (
    lib.findFirst (lib.hasPrefix "go ") (throw "go.mod has no go directive") (
      lib.splitString "\n" (builtins.readFile ../go.mod)
    )
  );
in
buildGoModule (finalAttrs: {
  pname = "skillshare";
  inherit version;

  # Unit tests read files across the repository (schemas, ui locales, scripts),
  # so exclude only what no Go code reads; docs edits then cause no rebuild.
  src = fileset.toSource {
    root = ../.;
    fileset = fileset.difference ../. (
      fileset.unions (
        map fileset.maybeMissing [
          ../ai_docs
          ../proposals
          ../video
          ../website
          ../wiki
          ../nix
          ../flake.nix
          ../flake.lock
          ../CHANGELOG.md
        ]
        ++ [ (fileset.fileFilter (file: lib.hasPrefix "README" file.name) ../.) ]
      )
    );
  };

  # Update when go.sum changes; the Nix CI job prints the new value.
  vendorHash = "sha256-vZxkLUP6E+tyat87VBpW3kr+JtXJizTHmEkiJDoZzbg=";

  # go.mod pins the newest Go patch release, which nixpkgs may not ship yet.
  # Lower only the patch; a newer minor needs a newer toolchain.
  postPatch =
    let
      goVersion = finalAttrs.passthru.go.version;
    in
    lib.optionalString (lib.versionOlder goVersion goModVersion) (
      assert lib.assertMsg (lib.versions.majorMinor goVersion == lib.versions.majorMinor goModVersion)
        "go.mod requires Go ${goModVersion}, but nixpkgs provides Go ${goVersion}; run `nix flake update`";
      ''
        go mod edit -go=${goVersion}
      ''
    );

  subPackages = [ "cmd/skillshare" ];

  env.CGO_ENABLED = 0;

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${finalAttrs.version}"
  ];

  nativeCheckInputs = [
    gitMinimal
    writableTmpDirAsHomeHook
  ];

  # Tests bind loopback servers.
  __darwinAllowLocalNetworking = true;

  # The sandbox has neither /bin/bash nor /usr/bin/env, which test scripts use.
  preCheck = ''
    patchShebangs .devcontainer/bin
    find cmd internal -name '*_test.go' -exec \
      sed -i -E 's@#!(/bin/bash|/usr/bin/env bash)@#!${runtimeShell}@' {} +
  '';

  # Installs skip the tests; the flake's checks enable them.
  doCheck = false;

  # Unit tests only, matching `scripts/test.sh --unit`. Tests run like upstream
  # CI: with cgo (some rely on its deadlock behavior) and without -trimpath
  # (some locate the repository through runtime.Caller).
  checkPhase = ''
    runHook preCheck
    CGO_ENABLED=1 go test -p $NIX_BUILD_CORES -trimpath=false ./cmd/... ./internal/...
    runHook postCheck
  '';

  nativeInstallCheckInputs = [ versionCheckHook ];
  versionCheckProgramArg = "version";
  doInstallCheck = true;

  meta = {
    description = "Share skills across AI CLI tools";
    homepage = "https://github.com/runkids/skillshare";
    changelog = "https://github.com/runkids/skillshare/releases/tag/v${finalAttrs.version}";
    license = lib.licenses.mit;
    mainProgram = "skillshare";
    platforms = lib.platforms.unix;
  };
})
