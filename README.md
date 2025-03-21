# LeetCode Grind Review Bot

A Discord bot to help users track their LeetCode problem-solving journey and schedule regular review sessions.

## Features

- Track solved LeetCode problems with difficulty, status, and category
- Add custom tags to problems for better organization
- Record if you solved a problem independently or needed hints
- View your problem-solving statistics 
- Get daily reminders to review previously solved problems
- Search and filter your problem history

## Installation

### Prerequisites

- Go 1.20 or higher
- SQLite3

### Setup

1. Clone the repository:
   ```
   git clone https://github.com/yugonline/grind_review_bot.git
   cd grind_review_bot
   ```

2. Install dependencies:
   ```
   go mod download
   ```

3. Configure the bot:
   - Copy `config/config.yaml` and customize the values
   - Set your Discord bot token (either in config or via environment variable `DISCORD_BOT_TOKEN`)

4. Build the bot:
   ```
   go build -o grind_review_bot ./cmd
   ```

5. Run the bot:
   ```
   ./grind_review_bot
   ```

## Discord Commands

- `/add` - Add a LeetCode problem you've solved
- `/list` - List your solved LeetCode problems
- `/get` - Get details of a solved problem by ID
- `/edit` - Edit an existing LeetCode problem
- `/delete` - Delete a solved problem by ID
- `/stats` - View your LeetCode problem solving statistics

## Docker Support

You can run the bot using Docker:

```
docker build -t grind_review_bot .
docker run -e DISCORD_BOT_TOKEN=your_token_here grind_review_bot
```

## Configuration

The bot can be configured via:
- `config.yaml` file
- Environment variables with the `GRIND_REVIEW_` prefix

Key configuration options:
- Discord bot token and guild ID
- Database connection settings
- Daily review reminder time
- Metrics server configuration

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.