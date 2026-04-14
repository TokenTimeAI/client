require "stripe"

Stripe.api_key = ENV["STRIPE_SECRET_KEY"] if ENV["STRIPE_SECRET_KEY"].present?
Stripe.max_network_retries = 2
