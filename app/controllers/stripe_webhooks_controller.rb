class StripeWebhooksController < ActionController::Base
  skip_forgery_protection

  def create
    if Billing::StripeConfiguration.webhook_secret.blank?
      Rails.logger.error("Stripe webhook received without STRIPE_WEBHOOK_SECRET configured")
      head :service_unavailable
      return
    end

    event = Stripe::Webhook.construct_event(
      request.raw_post,
      request.headers["HTTP_STRIPE_SIGNATURE"],
      Billing::StripeConfiguration.webhook_secret
    )

    Billing::StripeEventHandler.new.call(event)
    head :ok
  rescue Stripe::SignatureVerificationError, JSON::ParserError => e
    Rails.logger.warn("Invalid Stripe webhook: #{e.message}")
    head :bad_request
  rescue StandardError => e
    Rails.logger.error("Stripe webhook processing failed: #{e.class}: #{e.message}")
    head :unprocessable_entity
  end
end
