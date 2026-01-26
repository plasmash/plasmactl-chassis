# plasmactl-chassis

A [Launchr](https://github.com/launchrctl/launchr) plugin for [Plasmactl](https://github.com/plasmash/plasmactl) that manages the chassis structure for Plasma platforms.

## Overview

`plasmactl-chassis` manages the platform's "skeleton" - the structural framework where applications and flows attach. The chassis maps logical architecture to physical resources, ensuring components get appropriate compute resources (GPU for AI, storage for data, etc.).

## Concepts

### Chassis Structure

The chassis is defined in `chassis.yaml` as a hierarchical structure:

```yaml
platform:
  foundation:
    - cluster:
      - control
      - nodes
    - storage:
      - kv
    - network:
      - ingress
  interaction:
    - observability
    - management
  cognition:
    - data
    - knowledge
```

Each path (e.g., `platform.foundation.cluster.control`) represents a chassis section that:
- Can have nodes allocated to it
- Can have components attached to it
- Can have specific configuration in group_vars

## Commands

### chassis:list

List chassis sections from `chassis.yaml`:

```bash
# List all sections (flat)
plasmactl chassis:list

# List as tree
plasmactl chassis:list --tree

# List section and its children
plasmactl chassis:list platform.interaction
plasmactl chassis:list platform.foundation.cluster --tree
```

Options:
- `-t, --tree`: Show as tree instead of flat list

### chassis:show

Show details for a chassis section:

```bash
# Show section details
plasmactl chassis:show platform.interaction.observability

# Filter nodes by platform
plasmactl chassis:show platform.foundation.cluster.control --platform dev
```

Options:
- `-p, --platform`: Filter nodes by platform instance (default: all)

Output includes:
- Allocated nodes (from `inst/<platform>/nodes/`)
- Attached components (from layer playbooks)

### chassis:add

Add a new chassis section:

```bash
plasmactl chassis:add platform.interaction.analytics
plasmactl chassis:add platform.cognition.ml.training
```

### chassis:remove

Remove a chassis section:

```bash
plasmactl chassis:remove platform.interaction.legacy
```

**Safety**: Fails if nodes are allocated or components are attached. Use `node:allocate` and `component:detach` first to clean up.

## Directory Structure

The chassis interacts with several locations:

```
.
├── chassis.yaml                    # Chassis definition
├── inst/
│   └── <platform>/
│       └── nodes/
│           └── <hostname>.yaml     # Node allocations
└── src/
    └── <layer>/
        ├── <layer>.yaml            # Component attachments (playbooks)
        └── cfg/
            └── <chassis.section>/  # Section-specific configuration
                ├── vars.yaml
                └── vault.yaml
```

## Workflow Example

```bash
# 1. View current chassis structure
plasmactl chassis:list --tree

# 2. Add a new section for analytics
plasmactl chassis:add platform.interaction.analytics

# 3. Allocate nodes to the new section
plasmactl node:allocate node001 platform.interaction.analytics

# 4. Attach a component to the section
plasmactl component:attach interaction.applications.analytics platform.interaction.analytics

# 5. Verify the setup
plasmactl chassis:show platform.interaction.analytics

# 6. Remove a legacy section (after cleanup)
plasmactl node:allocate node001 platform.interaction.legacy-
plasmactl component:detach interaction.applications.old platform.interaction.legacy
plasmactl chassis:remove platform.interaction.legacy
```

## Related Commands

| Plugin | Command | Purpose |
|--------|---------|---------|
| plasmactl-node | `node:allocate` | Allocate nodes to chassis sections |
| plasmactl-component | `component:attach` | Attach components to chassis sections |
| plasmactl-component | `component:detach` | Detach components from chassis sections |

## Documentation

- [Plasmactl](https://github.com/plasmash/plasmactl) - Main CLI tool
- [plasmactl-node](https://github.com/plasmash/plasmactl-node) - Node management
- [plasmactl-component](https://github.com/plasmash/plasmactl-component) - Component management
- [Plasma Platform](https://plasma.sh) - Platform documentation

## License

[European Union Public License 1.2 (EUPL-1.2)](LICENSE)
