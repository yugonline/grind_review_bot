# Deploying Your Private Discord LeetCode Bot: Step-by-Step Guide

Here's a complete guide to deploy your private bot from scratch:

## 1. Create a Discord Bot

1. Go to [Discord Developer Portal](https://discord.com/developers/applications)
2. Click "New Application" and name it (e.g., "Grind Review Bot")
3. Go to the "Bot" tab and click "Add Bot"
4. Configure your bot to be private and secure:
   - **Toggle OFF "Public Bot"** (to make it private, only you can add it)
   - Under "Privileged Gateway Intents", enable:
     - SERVER MEMBERS INTENT (required to verify server membership)
     - MESSAGE CONTENT INTENT (required to read command content)
5. Copy your bot token (you'll need this later)
6. Go to "OAuth2" → "URL Generator"
7. Select scopes: `bot` and `applications.commands`
8. Bot permissions: 
   - `Send Messages`
   - `Use Slash Commands` 
   - `Read Messages/View Channels`
   - `Mention Everyone` (to tag everyone in review notifications)
9. Copy the generated URL and open it in your browser to invite the bot to your server
10. Create a channel named `#review_log` in your server - this will be the dedicated channel for bot commands

## 2. Set Up GitHub Repository

1. Create a new GitHub repository:
   ```bash
   git init
   git add .
   git commit -m "Initial commit"
   git branch -M main
   git remote add origin https://github.com/yugonline/grind_review_bot.git
   git push -u origin main
   ```

2. Set repository to private to keep your bot exclusive to your server

## 3. Install Development Requirements

1. Install Go (1.20 or later): [golang.org/dl](https://golang.org/dl/)
2. Install SQLite3: [sqlite.org/download.html](https://sqlite.org/download.html)

## 4. Configure the Bot

1. Edit `config/config.yaml`:
   ```yaml
   discord:
     token: "" # Leave empty, will be set via environment variable
     guild_id: "YOUR_DISCORD_SERVER_ID" # Required for private server-only bot
     review_channel_id: "YOUR_REVIEW_CHANNEL_ID" # Required - ID of the #review_log channel
     commands_timeout: 5s
     interaction_expiry: 15m

   database:
     driver: sqlite3
     dsn: ./data/grind_review.db # Store in the data directory for persistence
     # Other database settings remain unchanged

   scheduler:
     review_time: "08:00" # Set your preferred time for daily reminders
     review_channel: "YOUR_CHANNEL_ID" # Set the channel where reminders will be sent
     retry_attempts: 3
     retry_delay: 2s
     lookback_period: 24h

   metrics:
     enabled: false # Enable only if you need metrics
     address: ":9090"

   log_level: info
   ```

2. The module name in `go.mod` should be set to `github.com/yugonline/grind_review_bot`

## 5. Local Testing

1. Create a directory for your database:
   ```bash
   mkdir -p data
   ```

2. Set your Discord token and channel information as environment variables:
   ```bash
   export DISCORD_BOT_TOKEN=your_token_here
   export GRIND_REVIEW_DISCORD_GUILD_ID=your_server_id
   export GRIND_REVIEW_DISCORD_REVIEW_CHANNEL_ID=your_review_channel_id
   ```

3. Download dependencies:
   ```bash
   go mod download
   ```

4. Run the bot:
   ```bash
   go run ./cmd
   ```

5. Test the bot commands in your Discord server

## 6. Deployment Options

### Option 1: Direct Deployment (Linux/macOS)

1. Build the binary:
   ```bash
   go build -o grind_review_bot ./cmd
   ```

2. Make it executable:
   ```bash
   chmod +x grind_review_bot
   ```

3. Run the bot (with token):
   ```bash
   DISCORD_BOT_TOKEN=your_token_here ./grind_review_bot
   ```

4. For production, create a systemd service (Linux):
   ```
   [Unit]
   Description=LeetCode Grind Review Bot
   After=network.target

   [Service]
   Type=simple
   User=your_username
   WorkingDirectory=/path/to/bot
   ExecStart=/path/to/bot/grind_review_bot
   Restart=on-failure
   Environment=DISCORD_BOT_TOKEN=your_token_here
   Environment=GRIND_REVIEW_DISCORD_GUILD_ID=your_server_id
   Environment=GRIND_REVIEW_DISCORD_REVIEW_CHANNEL_ID=your_review_channel_id

   [Install]
   WantedBy=multi-user.target
   ```

   Save as `/etc/systemd/system/grind_review_bot.service`

   ```bash
   sudo systemctl enable grind_review_bot
   sudo systemctl start grind_review_bot
   ```

### Option 2: Docker Deployment

1. Build the Docker image:
   ```bash
   docker build -t grind_review_bot .
   ```

2. Create a volume for persistent data:
   ```bash
   docker volume create grind_review_data
   ```

3. Run the container:
   ```bash
   docker run -d \
     -e DISCORD_BOT_TOKEN=your_token_here \
     -e GRIND_REVIEW_DISCORD_GUILD_ID=your_server_id \
     -e GRIND_REVIEW_DISCORD_REVIEW_CHANNEL_ID=your_review_channel_id \
     -v grind_review_data:/app/data \
     --name grind_review_bot \
     --restart unless-stopped \
     grind_review_bot
   ```

### Option 3: Cloud Deployment (Railway.app)

1. Create an account on [Railway.app](https://railway.app/)
2. Create a new project and connect your GitHub repository
3. Add the environment variable: `DISCORD_BOT_TOKEN`
4. Deploy the service
5. Railway will handle builds and provide persistent storage

## 7. Bot Maintenance

1. View logs:
   - Direct: `tail -f logs/bot.log` (if logging to file)
   - Docker: `docker logs -f grind_review_bot`
   - SystemD: `journalctl -u grind_review_bot -f`

2. Updating the bot:
   ```bash
   git pull
   go build -o grind_review_bot ./cmd
   # Restart service or container
   ```

## Usage Notes

- The bot stores data in a SQLite database, which is perfect for personal/small Discord servers
- Each user's problems are stored separately by their Discord ID
- Commands only work in the designated #review_log channel (configured via review_channel_id)
- Only members of your server can use the commands (private bot functionality)
- The daily review scheduler will send reminders at the configured time
- The bot can tag everyone in the designated channel for reviews
- The bot is designed to be per-server, not public, so your LeetCode progress is private

## Security Notes

- Keep your bot token secret - never commit it to GitHub
- Use environment variables for sensitive information
- The bot only stores information for users who interact with it
- Data is stored locally, providing better privacy than cloud-based alternatives

This setup ensures your bot remains private to your Discord server while being easily maintainable. If you ever want to make it public, you'd need to consider database scaling, hosting costs, and adding user authentication.