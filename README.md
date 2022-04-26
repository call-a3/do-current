# DigitalOcean Current

This program will monitor a floating IP and kubernetes cluster in digital ocean and will make sure that said floating IP is assigned to one of the droplets that make up the worker nodes of said cluster.
This allows a dime-scratching skimper like me to save money on a load-balancer by using a floating IP instead.

## Downsides

- There is a possibility of some downtime when the droplet that the floating IP is assigned to crashes, is overloaded, is removed/replaced as for example happens during a kubernetes upgrade.

## Upsides

- You get a free static IP for your cluster! :tada:
- By combining this approach with services of the type `NodePort`, you can circumvent the limitation on DO loadbalancers that only support TCP-based protocols.
