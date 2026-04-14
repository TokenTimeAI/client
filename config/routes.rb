Rails.application.routes.draw do
  devise_for :users

  # Health check
  get "up" => "rails/health#show", as: :rails_health_check

  # Dashboard
  root "dashboard#index"

  # Web UI
  resources :projects
  resources :api_keys, only: %i[index create destroy]

  # API v1 — WakaTime-compatible endpoints for agent integrations
  namespace :api do
    namespace :v1 do
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
