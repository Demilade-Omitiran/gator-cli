module blog_aggregator

go 1.22.3

require internal/config v1.0.0

require (
	github.com/lib/pq v1.10.9
	internal/database v1.0.0
)

require github.com/google/uuid v1.6.0 // indirect

replace internal/config => ./internal/config

replace internal/database => ./internal/database
