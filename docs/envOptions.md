# Environment Variable Options

- `SHOW_CONTAINER_LOGS`: Controls whether container logs are displayed.

    - Set to `"always"` to show logs for both pass and fail.
    - Set to `"never"` to never show any logs.
    - Leave unset to show logs only for failed tests.

- `KEEP_CONTAINERS`: Prevents testnet cleanup after completion.

    - Set to any non-empty value to keep testnet containers alive.

- `CONTAINER_LOG_TAIL`: Specifies the number of lines to display from container logs. Defaults to 50 lines.

- `ICS_SPAWN_TIME_WAIT`: A go duration (e.g. 2m) that specifies how long to wait for ICS chains to spawn