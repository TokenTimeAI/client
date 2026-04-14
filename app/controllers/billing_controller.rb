class BillingController < ApplicationController
  def show
    @billing_config = Billing::StripeConfiguration
  end

  def checkout
    unless Billing::StripeConfiguration.checkout_ready?
      redirect_to billing_path, alert: "Billing is not configured yet."
      return
    end

    if current_user.subscription_active?
      redirect_to billing_path, notice: "You already have an active subscription. Manage it below."
      return
    end

    session = Stripe::Checkout::Session.create(
      mode: "subscription",
      success_url: "#{billing_url}?checkout=success",
      cancel_url: billing_url,
      customer: ensure_stripe_customer_id!,
      client_reference_id: current_user.id,
      metadata: {
        user_id: current_user.id
      },
      subscription_data: {
        metadata: {
          user_id: current_user.id
        }
      },
      line_items: [
        {
          price: Billing::StripeConfiguration.personal_monthly_price_id,
          quantity: 1
        }
      ]
    )

    redirect_to session.url, allow_other_host: true
  rescue Stripe::StripeError => e
    Rails.logger.error("Stripe checkout error for user #{current_user.id}: #{e.message}")
    redirect_to billing_path, alert: "We couldn't start checkout right now. Please try again."
  end

  def portal
    unless Billing::StripeConfiguration.portal_ready?
      redirect_to billing_path, alert: "Billing management is not configured yet."
      return
    end

    unless current_user.stripe_customer_id.present?
      redirect_to billing_path, alert: "Start a subscription first to manage billing."
      return
    end

    session = Stripe::BillingPortal::Session.create(
      customer: current_user.stripe_customer_id,
      return_url: billing_url
    )

    redirect_to session.url, allow_other_host: true
  rescue Stripe::StripeError => e
    Rails.logger.error("Stripe billing portal error for user #{current_user.id}: #{e.message}")
    redirect_to billing_path, alert: "We couldn't open billing management right now. Please try again."
  end

  private

  def ensure_stripe_customer_id!
    return current_user.stripe_customer_id if current_user.stripe_customer_id.present?

    customer = Stripe::Customer.create(
      email: current_user.email,
      name: current_user.display_name.presence || current_user.email,
      metadata: {
        user_id: current_user.id
      }
    )

    current_user.update!(stripe_customer_id: customer.id)
    customer.id
  end
end
