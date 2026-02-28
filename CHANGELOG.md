# Changelog

All notable changes to the Dotfile Agent project.

## [Unreleased]

### Added
- **Automatic Software Installation**: Enhanced syncer now automatically installs required software during sync
  - Detects platform and installs appropriate packages
  - Continues sync even if some packages fail to install
  - Logs installation progress and results
  - New sync step: "Install required software"

### Changed
- Replaced all `fmt.Println` calls with `Infoln` for consistent logging with timestamps
- Replaced informational `fmt.Printf` calls with `Infoln` where appropriate
- Standardized error logging to use `Error()` function throughout codebase
- Enhanced syncer now has 4 steps instead of 3 (added software installation)

### Details

#### Software Installation Feature
- **enhanced_syncer.go**: Added automatic software installation step
  - New step runs after config parsing, before file copying
  - Detects platform using `GetPlatform()`
  - Executes installation commands for current platform
  - Logs success/failure for each package
  - Continues sync even if installations fail
  - Provides summary of installation results

#### Logging Improvements
- **broker.go**: Replaced `fmt.Println(err)` with `Error(err.Error())` for consistent error logging
- **install_software.go**: 
  - Replaced platform detection messages with `Infoln`
  - Replaced installation progress messages with `Infoln`
  - Replaced success/failure messages with `Infoln`/`Error`
  - Kept `fmt.Print` for interactive user prompts (no timestamp needed)
  - Kept `fmt.Printf` for formatted list output (indented items)

#### Benefits
- All log messages now include RFC3339 timestamps
- Consistent log format across the entire application
- Easier to parse logs for monitoring and debugging
- Clear distinction between INFO and ERROR level messages

### Files Modified
- `enhanced_syncer.go` - Added software installation step
- `broker.go` - Logging improvements
- `install_software.go` - Logging improvements
- `README.md` - Updated documentation for automatic installation
- `CHANGELOG.md` - Documented changes

### Files Unchanged
- `io.go` - Contains the logging function implementations
- Other files - Already using proper logging functions

## Previous Changes

### Added
- Enhanced configuration format with platform-specific installations
- Docker support with multi-stage builds
- Comprehensive documentation (README.md, DOCKER_TESTING.md, CODEBASE_OVERVIEW.md)
- Ubuntu setup script with systemd service configuration
- Software installation utilities
- Platform detection and cross-platform support

### Documentation
- Refactored README.md with comprehensive examples and API documentation
- Added DOCKER_TESTING.md for container testing
- Added CODEBASE_OVERVIEW.md for architecture documentation
- Added inline comments to all functions, structs, and methods
