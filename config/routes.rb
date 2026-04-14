Rails.application.routes.draw do
  devise_for :users, controllers: { omniauth_callbacks: "users/omniauth_callbacks" }

  # Health check
  get "up" => "rails/health#show", as: :rails_health_check

  # Install scripts
  get "install.sh", to: "install_scripts#install"
  get "install.ps1", to: "install_scripts#windows"

  # Dashboard
  root "dashboard#index"

  # Web UI
  resources :projects
  resources :api_keys, only: %i[index create destroy]
  get "device/:user_code", to: "device_authorizations#show", as: :device_authorization
  post "device/:user_code/approve", to: "device_authorizations#approve", as: :approve_device_authorization
  resource :billing, only: :show, controller: "billing" do
    post :checkout
    post :portal
  end
  post "stripe/webhooks", to: "stripe_webhooks#create"

  # API v1 — WakaTime-compatible endpoints for agent integrations
  namespace :api do
    namespace :v1 do
      post "device_authorizations", to: "device_authorizations#create"
      post "device_authorizations/poll", to: "device_authorizations#poll"

      # Release checking
      get "releases/latest", to: "releases#latest"

      # Heartbeat ingestion (WakaTime-compatible)
      post "heartbeats", to: "heartbeats#create"
      post "users/current/heartbeats", to: "heartbeats#create"
      post "heartbeats/bulk", to: "heartbeats#bulk"
      post "users/current/heartbeats/bulk", to: "heartbeats#bulk"

      # User info
      get "users/current", to: "users#current"

      # Summaries & stats
      get "summaries", to: "summaries#index"
      get "users/current/summaries", to: "summaries#index"

      # Status bar (duration today)
      get "users/current/statusbar/today", to: "status_bar#today"

      # Projects
      resources :projects, only: %i[index show]
    end
  end
end
