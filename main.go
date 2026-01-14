package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/google/go-github/v72/github"
	"golang.org/x/oauth2"
)

func main() {
	noPrompt := flag.Bool("no-prompt", false, "Skip confirmation prompts")
	markDone := flag.Bool("mark-done", false, "Mark notifications as done")
	onlyRepos := flag.String("only-repos", "", "Comma-separated list of repositories to include (owner/repo)")
	excludeRepos := flag.String("exclude-repos", "", "Comma-separated list of repositories to exclude (owner/repo)")
	concurrency := flag.Int("concurrency", 1, "Number of concurrent workers for processing notifications")
	actorFilter := flag.String("actor", "", "Only process notifications for PRs created by this actor (username)")
	flag.Parse()

	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("Please set the GITHUB_TOKEN environment variable.")
	}

	var onlyReposSet, excludeReposSet map[string]struct{}
	if *onlyRepos != "" {
		onlyReposSet = make(map[string]struct{})
		for _, repo := range strings.Split(*onlyRepos, ",") {
			onlyReposSet[strings.TrimSpace(repo)] = struct{}{}
		}
	}
	if *excludeRepos != "" {
		excludeReposSet = make(map[string]struct{})
		for _, repo := range strings.Split(*excludeRepos, ",") {
			excludeReposSet[strings.TrimSpace(repo)] = struct{}{}
		}
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	markingBehavior := "read"
	if *markDone {
		markingBehavior = "done"
	}

	client := github.NewClient(oauth2.NewClient(ctx, ts))

	allNotifications := getUnreadNotifications(client, ctx)

	var prNotifications []*github.Notification
	for _, notification := range allNotifications {
		subject := notification.GetSubject()
		if subject.GetType() == "PullRequest" {
			repoFullName := strings.TrimPrefix(notification.GetRepository().GetFullName(), "repos/")

			if onlyReposSet != nil {
				if _, ok := onlyReposSet[repoFullName]; !ok {
					continue
				}
			}
			if excludeReposSet != nil {
				if _, ok := excludeReposSet[repoFullName]; ok {
					continue
				}
			}

			prNotifications = append(prNotifications, notification)
		}
	}

	processNotifications(client, ctx, prNotifications, allNotifications, *concurrency, *noPrompt, *markDone, markingBehavior, *actorFilter)
}

func getUnreadNotifications(client *github.Client, ctx context.Context) []*github.Notification {
	var allNotifications []*github.Notification
	opt := &github.NotificationListOptions{
		All:         false, // only include unread notifications
		ListOptions: github.ListOptions{PerPage: 50},
	}

	totalFetched := 0
	page := 1

	fmt.Printf("Fetching all GitHub notifications\n")
	for {
		opt.Page = page
		notifications, resp, err := client.Activity.ListNotifications(ctx, opt)
		if err != nil {
			log.Fatalf("Error fetching GH notifications: %v", err)
		}

		allNotifications = append(allNotifications, notifications...)
		totalFetched += len(notifications)

		fmt.Printf("\r📦 Page %d — Total fetched: %d notifications", page, totalFetched)

		if resp.NextPage == 0 {
			break
		}
		page++
	}

	fmt.Printf("\n✅ Done! Fetched %d notifications in total.\n", totalFetched)
	return allNotifications
}

func markNotificationRead(client *github.Client, ctx context.Context, threadID string) {
	_, err := client.Activity.MarkThreadRead(ctx, threadID)
	if err != nil {
		log.Printf("    ❌ Failed to mark thread %s as read: %v\n", threadID, err)
	} else {
		fmt.Printf("    🟢 Successfully marked thread as read\n")
	}
}

func markNotificationDone(client *github.Client, ctx context.Context, threadID string) {
	threadIDInt, err := strconv.Atoi(threadID)
	if err != nil {
		log.Printf("    ❌ Failed to convert thread ID %s to int: %v\n", threadID, err)
		return
	}
	_, err = client.Activity.MarkThreadDone(ctx, int64(threadIDInt))
	if err != nil {
		log.Printf("    ❌ Failed to mark thread %s as done: %v\n", threadID, err)
	} else {
		fmt.Printf("    ✅ Successfully marked thread %s as done\n", threadID)
	}
}

func getNotificationThreadID(notifications []*github.Notification, owner string, repo string, prNumber string) string {
	for _, n := range notifications {
		if strings.Contains(n.GetSubject().GetURL(), fmt.Sprintf("/repos/%s/%s/pulls/%s", owner, repo, prNumber)) {
			return n.GetID()
		}
	}

	return ""
}

func atoi(s string) int {
	v, _ := strconv.Atoi(s)
	return v
}

func promptProceed(skip bool) bool {
	if skip {
		return true
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Continue: ? (y/n): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "", "y":
			return true
		case "n":
			return false
		default:
			fmt.Println("Invalid input, please enter 'y' or 'n', or just press Enter for yes.")
		}
	}
}

func apiToWebURL(apiURL string) string {
	// Convert API URL to GitHub Web URL
	url := strings.Replace(apiURL, "https://api.github.com/repos/", "https://github.com/", 1)
	return strings.Replace(url, "/pulls/", "/pull/", 1)
}

func processNotifications(client *github.Client, ctx context.Context, prNotifications []*github.Notification, allNotifications []*github.Notification, concurrency int, noPrompt bool, markDone bool, markingBehavior string, actorFilter string) {
	if len(prNotifications) == 0 {
		return
	}

	semaphore := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, notification := range prNotifications {
		wg.Add(1)
		go func(n *github.Notification) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			processNotification(client, ctx, n, allNotifications, noPrompt, markDone, markingBehavior, actorFilter)
		}(notification)
	}

	wg.Wait()
}

func processNotification(client *github.Client, ctx context.Context, notification *github.Notification, allNotifications []*github.Notification, noPrompt bool, markDone bool, markingBehavior string, actorFilter string) {
	subject := notification.GetSubject()
	prURL := subject.GetURL()
	parts := strings.Split(prURL, "/")
	owner := parts[4]
	repo := parts[5]
	prNumber := parts[len(parts)-1]
	prBrowserFriendlyURL := apiToWebURL(prURL)

	repoFullName := strings.TrimPrefix(notification.GetRepository().GetFullName(), "repos/")
	ownerRepo := strings.Split(repoFullName, "/")
	pr, _, err := client.PullRequests.Get(ctx, ownerRepo[0], ownerRepo[1], atoi(prNumber))
	if err != nil {
		log.Printf("Error fetching PR %s: %v\n", prURL, err)
		return
	}

	prTitle := pr.GetTitle()
	prAuthor := pr.GetUser().GetLogin()
	prIsMerged := pr.GetMerged()
	prIsClosed := pr.GetState() == "closed"
	notificationIsRead := !notification.GetUnread()

	// Skip if actor filter is set and doesn't match
	if actorFilter != "" && !strings.EqualFold(prAuthor, actorFilter) {
		return
	}

	if prIsMerged || prIsClosed {
		if notificationIsRead {
			return
		}
		fmt.Printf("🟡 PR: %s, Author: @%s, Title: \"%s\", is merged or closed and notification will be marked as %s\n", prBrowserFriendlyURL, prAuthor, prTitle, markingBehavior)

		threadID := getNotificationThreadID(allNotifications, owner, repo, prNumber)
		if threadID != "" {
			fmt.Printf("  🟡 \033[33mAbout to mark related GH Notification with threadID: \"%s\" as *%s*\033[0m\n", threadID, strings.ToUpper(markingBehavior))

			if promptProceed(noPrompt) {
				if markDone {
					markNotificationDone(client, ctx, threadID)
				} else {
					markNotificationRead(client, ctx, threadID)
				}
			}
		}
	} else {
		fmt.Printf("PR: %s, Author: @%s, Title: \"%s\", is unmerged and waiting for your review!\n", prBrowserFriendlyURL, prAuthor, prTitle)
	}
}
