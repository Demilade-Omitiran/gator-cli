# GATOR CLI
This is a Blog-Aggregator (or `gator`) CLI.

To run this,
- You need PostgreSQL installed (>= version 15).
- Create a database for the application.
- Create the `.gatorconfig.json` file in your home directory, and add the following to the file
```
  {"db_url":"DATABASE_URL?sslmode=disable"}
```
- Run `go install`

To get a list of runnable commands, run `go run . help`.