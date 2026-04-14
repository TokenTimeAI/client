module Billing
  class StripeEventHandler
    def call(event)
      case event.type
      when "checkout.session.completed"
        handle_checkout_session_completed(event.data.object)
      when "customer.subscription.created", "customer.subscription.updated"
        handle_subscription_updated(event.data.object)
      when "customer.subscription.deleted"
        handle_subscription_deleted(event.data.object)
      else
        Rails.logger.info("Ignoring unhandled Stripe event type=#{event.type}")
      end
    end

    private

    def handle_checkout_session_completed(session)
      return unless session.mode == "subscription"

      user = user_from_checkout_session(session)
      return unless user

      subscription = checkout_subscription(session)
      user.sync_stripe_subscription!(subscription) if subscription

      updates = {
        stripe_customer_id: session.customer,
        stripe_subscription_id: session.subscription
      }.compact

      user.update!(updates) if updates.any?
    end

    def handle_subscription_updated(subscription)
      user = user_from_subscription(subscription)
      return unless user

      user.sync_stripe_subscription!(subscription)
    end

    def handle_subscription_deleted(subscription)
      user = user_from_subscription(subscription)
      return unless user

      user.mark_subscription_canceled!(subscription)
    end

    def user_from_checkout_session(session)
      User.find_by(id: session.client_reference_id) ||
        User.find_by(stripe_customer_id: session.customer) ||
        User.find_by(email: session.customer_details&.email || session.customer_email)
    end

    def user_from_subscription(subscription)
      metadata_user_id = subscription.metadata&.[]("user_id") || subscription.metadata&.[](:user_id)

      User.find_by(id: metadata_user_id) ||
        User.find_by(stripe_customer_id: subscription.customer) ||
        User.find_by(stripe_subscription_id: subscription.id)
    end

    def checkout_subscription(session)
      return session.subscription if session.subscription.respond_to?(:status)
      return unless session.subscription.present?

      Stripe::Subscription.retrieve(session.subscription)
    rescue Stripe::StripeError => e
      Rails.logger.warn("Unable to hydrate checkout subscription #{session.subscription}: #{e.message}")
      nil
    end
  end
end
