package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"internal/config"
	"internal/database"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type state struct {
	config *config.Config
	db     *database.Queries
}

type command struct {
	name string
	args []string
}

type commands struct {
	commandMap map[string]func(*state, command) error
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type supportedCommand struct {
	name        string
	description string
	params      []string
}

func getCurrentUser(s *state) (database.User, error) {
	currentUserName := s.config.CurrentUserName

	if currentUserName == "" {
		return database.User{}, fmt.Errorf("no user logged in")
	}

	currentUser, err := s.db.GetUser(context.Background(), currentUserName)

	if err != nil {
		return database.User{}, err
	}

	return currentUser, nil
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		currentUser, err := getCurrentUser(s)

		if err != nil {
			return err
		}

		err = handler(s, cmd, currentUser)

		if err != nil {
			return err
		}

		return nil
	}
}

func handlerLogin(s *state, cmd command) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("username is required")
	}

	username := cmd.args[0]

	if _, err := s.db.GetUser(context.Background(), username); err != nil {
		fmt.Printf("%v\n", err)
		return fmt.Errorf("user not found")
	}

	s.config.SetUser(username)

	fmt.Println("user has been set")

	return nil
}

func handlerRegister(s *state, cmd command) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("name is required")
	}

	name := cmd.args[0]

	contextBackground := context.Background()

	creationParams := database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      name,
	}

	dbUser, err := s.db.CreateUser(contextBackground, creationParams)

	if err != nil {
		return fmt.Errorf("error creating user")
	}

	s.config.SetUser(name)

	fmt.Println("user has been set")
	fmt.Println(dbUser)

	return nil
}

func handlerReset(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("invalid number of params")
	}

	if err := s.db.Reset(context.Background()); err != nil {
		return fmt.Errorf("error running reset command")
	}

	return nil
}

func handlerUsers(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("invalid number of params")
	}

	users, err := s.db.GetUsers(context.Background())

	if err != nil {
		return fmt.Errorf("error fetching users")
	}

	for _, user := range users {
		stringToDisplay := "* " + user.Name

		if user.Name == s.config.CurrentUserName {
			stringToDisplay += " (current)"
		}

		fmt.Printf("%s\n", stringToDisplay)
	}

	return nil
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	request, err := http.NewRequestWithContext(ctx, "GET", feedURL, nil)

	if err != nil {
		return &RSSFeed{}, err
	}

	fmt.Printf("Making request to %s\n", feedURL)

	request.Header.Set("User-Agent", "gator")

	client := http.Client{}

	response, err := client.Do(request)

	if err != nil {
		return &RSSFeed{}, err
	}

	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)

	if err != nil {
		return &RSSFeed{}, err
	}

	var rssFeed RSSFeed

	if err = xml.Unmarshal(data, &rssFeed); err != nil {
		return &RSSFeed{}, err
	}

	rssFeed.Channel.Title = html.UnescapeString(rssFeed.Channel.Title)
	rssFeed.Channel.Description = html.UnescapeString(rssFeed.Channel.Description)

	for _, item := range rssFeed.Channel.Item {
		item.Title = html.UnescapeString(item.Title)
		item.Description = html.UnescapeString(item.Description)
	}

	return &rssFeed, nil
}

func scrapeFeeds(s *state) error {
	nextFeed, err := s.db.GetNextFeedToFetch(context.Background())

	if err != nil {
		return err
	}

	err = s.db.MarkFeedFetched(context.Background(), database.MarkFeedFetchedParams{
		ID:        nextFeed.ID,
		UpdatedAt: time.Now(),
	})

	if err != nil {
		return err
	}

	fetchedFeed, err := fetchFeed(context.Background(), nextFeed.Url)

	if err != nil {
		return err
	}

	for _, item := range fetchedFeed.Channel.Item {
		fmt.Printf("- Title: %s\n", item.Title)

		publishedAt, _ := time.Parse(time.UnixDate, item.PubDate)

		_, err := s.db.CreatePost(context.Background(), database.CreatePostParams{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			Title: sql.NullString{
				String: item.Title,
				Valid:  true,
			},
			Url: item.Link,
			Description: sql.NullString{
				String: item.Description,
				Valid:  true,
			},
			PublishedAt: publishedAt,
			FeedID:      nextFeed.ID,
		})

		if err != nil {
			fmt.Println(err.Error())
		}
	}

	fmt.Println("----------------------------------------------------------------------------------------------------------------")

	return nil
}

func handlerAgg(s *state, cmd command) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("invalid number of params")
	}

	timeBetweenReqs := cmd.args[0]

	parsedTimeBetweenReqs, err := time.ParseDuration(timeBetweenReqs)

	if err != nil {
		return err
	}

	ticker := time.NewTicker(parsedTimeBetweenReqs)

	defer ticker.Stop()

	fmt.Printf("Collecting feeds every %v\n", timeBetweenReqs)

	for ; ; <-ticker.C {
		scrapeFeeds(s)
	}
}

func handlerAddFeed(s *state, cmd command, currentUser database.User) error {
	if len(cmd.args) != 2 {
		return fmt.Errorf("invalid number of params")
	}

	name := cmd.args[0]
	url := cmd.args[1]

	creationParams := database.CreateFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Name:      name,
		Url:       url,
		UserID:    currentUser.ID,
	}

	feed, err := s.db.CreateFeed(context.Background(), creationParams)

	if err != nil {
		return err
	}

	feedFollowParams := database.CreateFeedFollowParams{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    currentUser.ID,
		FeedID:    feed.ID,
	}

	_, err = s.db.CreateFeedFollow(context.Background(), feedFollowParams)

	if err != nil {
		return err
	}

	fmt.Printf("%v\n", feed)

	return nil
}

func handlerGetFeeds(s *state, cmd command) error {
	if len(cmd.args) > 0 {
		return fmt.Errorf("invalid number of params")
	}

	feeds, err := s.db.GetFeeds(context.Background())

	if err != nil {
		return fmt.Errorf("error fetching users")
	}

	for _, feed := range feeds {
		fmt.Printf("%v", feed)
	}

	return nil
}

func handlerFollow(s *state, cmd command, currentUser database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("invalid number of params")
	}

	url := cmd.args[0]

	feed, err := s.db.GetFeed(context.Background(), url)

	if err != nil {
		return err
	}

	feedFollowParams := database.CreateFeedFollowParams{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		UserID:    currentUser.ID,
		FeedID:    feed.ID,
	}

	feedFollow, err := s.db.CreateFeedFollow(context.Background(), feedFollowParams)

	if err != nil {
		return err
	}

	fmt.Printf("User Name: %s, Feed Name: %s\n", feedFollow.UserName, feedFollow.FeedName)

	return nil
}

func handlerFollowing(s *state, cmd command, currentUser database.User) error {
	if len(cmd.args) != 0 {
		return fmt.Errorf("invalid number of params")
	}

	feedFollows, err := s.db.GetFeedFollowsForUser(context.Background(), currentUser.ID)

	if err != nil {
		return err
	}

	for _, feedFollow := range feedFollows {
		fmt.Printf("- User Name: %s, Feed Name: %s\n", feedFollow.UserName, feedFollow.FeedName)
	}

	return nil
}

func handlerUnfollow(s *state, cmd command, currentUser database.User) error {
	if len(cmd.args) != 1 {
		return fmt.Errorf("invalid number of params")
	}

	url := cmd.args[0]

	feed, err := s.db.GetFeed(context.Background(), url)

	if err != nil {
		return err
	}

	unfollowParams := database.DeleteFeedFollowParams{
		UserID: currentUser.ID,
		FeedID: feed.ID,
	}

	if err = s.db.DeleteFeedFollow(context.Background(), unfollowParams); err != nil {
		return err
	}

	return nil
}

func handlerBrowse(s *state, cmd command, currentUser database.User) error {
	if len(cmd.args) > 1 {
		return fmt.Errorf("invalid number of params")
	}

	limit := 2

	if len(cmd.args) == 1 {
		if val, err := strconv.Atoi(cmd.args[0]); err == nil {
			limit = val
		}
	}

	posts, err := s.db.GetPostsForUser(context.Background(), database.GetPostsForUserParams{
		UserID: currentUser.ID,
		Limit:  int32(limit),
	})

	if err != nil {
		return err
	}

	for _, post := range posts {
		fmt.Printf("- %v\n", post)
	}

	return nil
}

func handlerHelp(s *state, cmd command) error {
	fmt.Printf("GATOR CLI!\n")

	fmt.Println("Supported Commands")

	supportedCommands := []supportedCommand{
		{
			name:        "login",
			description: "logs a user in",
			params:      []string{"name"},
		},
		{
			name:        "register",
			description: "registers and logs in a user",
			params:      []string{"name"},
		},
		{
			name:        "reset",
			description: "clears the database",
			params:      []string{},
		},
		{
			name:        "users",
			description: "fetches all the users",
			params:      []string{},
		},
		{
			name:        "agg",
			description: "does the aggregation of blogs from feeds in the database",
			params:      []string{"time between requests (to fetch feeds)"},
		},
		{
			name:        "addfeed",
			description: "adds a feed for the currently logged-in user (user-login required)",
			params:      []string{"feed name", "feed url"},
		},
		{
			name:        "feeds",
			description: "fetches and prints feeds",
			params:      []string{},
		},
		{
			name:        "follow",
			description: "follows a feed for the currently logged-in user (user-login required)",
			params:      []string{"feed url"},
		},
		{
			name:        "following",
			description: "prints all feeds being followed by the currently logged-in user (user-login required)",
			params:      []string{},
		},
		{
			name:        "unfollow",
			description: "unfollows a feed for the currently logged-in user (user-login required)",
			params:      []string{"feed url"},
		},
		{
			name:        "browse",
			description: "fetches feed posts for the currently logged-in user (user-login required)",
			params:      []string{"number of posts (optional)"},
		},
		{
			name:        "help",
			description: "displays all the supported commands",
			params:      []string{},
		},
	}

	for _, el := range supportedCommands {
		fmt.Printf("command: %s\ndescription: %s\n", el.name, el.description)

		if len(el.params) > 0 {
			fmt.Println("params:")

			for _, param := range el.params {
				fmt.Printf("- %s\n", param)
			}
		}

		fmt.Printf("\n")
	}

	return nil
}

func (c *commands) run(s *state, cmd command) error {
	commandFunc, ok := c.commandMap[cmd.name]

	if !ok {
		return fmt.Errorf("invalid command")
	}

	if err := commandFunc(s, cmd); err != nil {
		return err
	}

	return nil
}

func (c *commands) register(name string, f func(*state, command) error) {
	c.commandMap[name] = f
}

func main() {
	jsonConfig, err := config.Read()

	if err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}

	mainState := state{
		config: &jsonConfig,
	}

	mainCommands := commands{
		commandMap: make(map[string]func(*state, command) error),
	}

	dbURL := jsonConfig.DbUrl

	db, err := sql.Open("postgres", dbURL)

	if err != nil {
		fmt.Println("Connection error!")
		os.Exit(1)
	}

	mainState.db = database.New(db)

	mainCommands.register("login", handlerLogin)
	mainCommands.register("register", handlerRegister)
	mainCommands.register("reset", handlerReset)
	mainCommands.register("users", handlerUsers)
	mainCommands.register("agg", handlerAgg)
	mainCommands.register("addfeed", middlewareLoggedIn(handlerAddFeed))
	mainCommands.register("feeds", handlerGetFeeds)
	mainCommands.register("follow", middlewareLoggedIn(handlerFollow))
	mainCommands.register("following", middlewareLoggedIn(handlerFollowing))
	mainCommands.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	mainCommands.register("browse", middlewareLoggedIn(handlerBrowse))
	mainCommands.register("help", handlerHelp)

	mainArgs := os.Args

	if len(mainArgs) < 2 {
		fmt.Println("Error: invalid number of arguments")
		os.Exit(1)
	}

	mainArgs = mainArgs[1:]

	mainCommand := command{
		name: mainArgs[0],
		args: mainArgs[1:],
	}

	err = mainCommands.run(&mainState, mainCommand)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
