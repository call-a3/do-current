package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"time"

	"github.com/digitalocean/godo"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
)

func main() {
	debug := flag.Bool("debug", false, "sets log level to debug")
	flag.Parse()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	ctx := context.Background()

	token := os.Getenv("DO_API_TOKEN")
	errors := 0
	if token == "" {
		log.Fatal().Msg("Environment variable DO_API_TOKEN is required")
		errors++
	}

	ip_address := os.Getenv("DO_FLOATING_IP")
	if ip_address == "" {
		log.Fatal().Msg("Environment variable DO_FLOATING_IP is required")
		errors++
	}

	cluster_id := os.Getenv("DO_CLUSTER_ID")
	if cluster_id == "" {
		log.Fatal().Msg("Environment variable DO_CLUSTER_ID is required")
		errors++
	}

	if errors > 0 {
		os.Exit(errors)
	}

	tokensource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	client := godo.NewClient(oauth2.NewClient(ctx, tokensource))

	for {
		floating_ip, _, err := client.FloatingIPs.Get(ctx, ip_address)
		if err != nil {
			log.Fatal().Err(err).Msg("Could not find floating IP")
			os.Exit(100)
		}
		// fmt.Fprintf(os.Stdout, "Found floating IP: %s\n", floating_ip.IP)
		log.Debug().Str("floating-ip", floating_ip.IP).Msg("Found floating IP")

		node_pools, response, err := client.Kubernetes.ListNodePools(ctx, cluster_id, &godo.ListOptions{})
		if err != nil {
			log.Fatal().Err(err).Msg("Could not find node pools of kubernetes cluster")
			os.Exit(101)
		}
		log.Debug().Interface("nodepools", node_pools).Msg("Retrieved node pools")
		next_droplet_id := 0
		should_flip_change := true
		done := false

		for _, node_pool := range node_pools {
			log.Debug().Interface("nodepool", node_pool).Msg("Processing nodepool")
			for _, node := range node_pool.Nodes {
				log.Debug().Interface("nodepool", node_pool).Interface("node", node).Msg("Processing node")
				parsed_droplet_id, err := strconv.Atoi(node.DropletID)
				next_droplet_id = parsed_droplet_id
				if err != nil {
					log.Warn().Err(err).Msg("Could not parse droplet id")
					break
				}
				if floating_ip.Droplet == nil {
					// fmt.Fprintf(os.Stdout, "Floating IP %s is currently unassigned", floating_ip.IP)
					log.Debug().Str("floating-ip", floating_ip.IP).Str("cluster-id", cluster_id).Int("droplet-id", next_droplet_id).Msg("Floating IP is currently unassigned")
					should_flip_change = true
					done = true
				} else if next_droplet_id == floating_ip.Droplet.ID {
					// fmt.Fprintf(os.Stdout, "Floating IP %s is already assigned to droplet %d in cluster %s\n", floating_ip.IP, floating_ip.Droplet.ID, cluster_id)
					log.Info().Str("floating-ip", floating_ip.IP).Str("cluster-id", cluster_id).Int("droplet-id", floating_ip.Droplet.ID).Msg("Floating IP is already assigned to a droplet in the cluster")
					should_flip_change = false
					done = true
				}
				if done {
					break
				}
			}
			if done {
				break
			}
		}

		if should_flip_change {
			log.Debug().Str("floating-ip", floating_ip.IP).Int("droplet-id", next_droplet_id).Msg("Floating IP is about to change")
			var action *godo.Action
			var err error
			if next_droplet_id != 0 {
				log.Info().Str("floating-ip", floating_ip.IP).Int("droplet-id", next_droplet_id).Msg("Assigning floating IP to a(nother) droplet")
				action, _, err = client.FloatingIPActions.Assign(ctx, floating_ip.IP, next_droplet_id)
			} else if floating_ip.Droplet != nil {
				log.Info().Str("floating-ip", floating_ip.IP).Msg("Unassigning floating IP because cluster has no droplets")
				action, _, err = client.FloatingIPActions.Unassign(ctx, floating_ip.IP)
			}

			if err != nil {
				log.Error().Err(err).Msg("Could not perform action on floating IP")
			}

			if action != nil {
				for {
					action, response, err = client.FloatingIPActions.Get(ctx, floating_ip.IP, action.ID)
					if err != nil {
						log.Error().Err(err).Msg("Could not check status of action on floating IP")
					}
					if action.Status != "in-progress" {
						break
					}
					log.Trace().Int("action-id", action.ID).Msg("Waiting until action on floating IP completes")
					if response.Rate.Remaining <= 0 {
						time.Sleep(response.Rate.Reset.Sub(time.Now()))
					} else {
						time.Sleep(10 * time.Second)
					}
				}
			}
		}

		duration_until_reset := response.Rate.Reset.Sub(time.Now())
		delay_until_next_cycle := 1 * time.Minute
		if response.Rate.Remaining <= 0 {
			delay_until_next_cycle = duration_until_reset + 5*time.Second
			// fmt.Fprintf(os.Stdout, "Waiting for %s until the rate limit has been reset\n", duration_until_reset.String())
			log.Info().Str("sleep-duration", delay_until_next_cycle.String()).Msg("Waiting until the rate limit has been reset")
		} else {
			possible_cycles := response.Rate.Remaining / 5
			delay_until_next_cycle = longestDuration(duration_until_reset/time.Duration(possible_cycles), 1*time.Minute)
			// fmt.Fprintf(os.Stdout, "Waiting for %s before checking again.\n", delay_until_next_cycle.String())
			log.Info().Str("sleep-duration", delay_until_next_cycle.String()).Msg("Waiting before checking floating IP assignment again")
		}
		time.Sleep(delay_until_next_cycle)
	}
}

func longestDuration(a time.Duration, b time.Duration) time.Duration {
	if a >= b {
		return a
	} else {
		return b
	}
}
