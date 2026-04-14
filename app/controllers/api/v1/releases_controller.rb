# frozen_string_literal: true

module Api
  module V1
    class ReleasesController < BaseController
      skip_before_action :authenticate_api_key!, only: [:latest]

      def latest
        # Get latest release from GitHub API
        release = fetch_github_release

        if release
          render json: {
            version: release["tag_name"],
            published_at: release["published_at"],
            assets: release_assets(release)
          }
        else
          render json: { error: "Release not found" }, status: :not_found
        end
      end

      private

      def fetch_github_release
        cache_key = "github_latest_release"
        Rails.cache.fetch(cache_key, expires_in: 5.minutes) do
          response = Net::HTTP.get(URI("https://api.github.com/repos/tokentimeai/client/releases/latest"))
          JSON.parse(response)
        rescue StandardError
          nil
        end
      end

      def release_assets(release)
        return [] unless release["assets"]

        release["assets"].filter_map do |asset|
          next unless asset["name"].match?(/ttime_(Darwin|Linux|Windows)_(amd64|arm64)\.(zip|tar\.gz)/)

          {
            name: asset["name"],
            url: asset["browser_download_url"],
            size: asset["size"],
            platform: platform_from_filename(asset["name"]),
            arch: arch_from_filename(asset["name"])
          }
        end
      end

      def platform_from_filename(filename)
        case filename
        when /Darwin/ then "darwin"
        when /Linux/ then "linux"
        when /Windows/ then "windows"
        end
      end

      def arch_from_filename(filename)
        case filename
        when /amd64/ then "amd64"
        when /arm64/ then "arm64"
        end
      end
    end
  end
end
