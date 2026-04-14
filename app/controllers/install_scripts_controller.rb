# frozen_string_literal: true

class InstallScriptsController < ApplicationController
  skip_before_action :authenticate_user!, only: %i[install windows]
  skip_before_action :verify_authenticity_token, only: %i[install windows]

  def install
    serve_script("install.sh")
  end

  def windows
    serve_script("install.ps1")
  end

  private

  def serve_script(filename)
    script_path = Rails.root.join("public", filename)

    if File.exist?(script_path)
      send_file script_path,
                type: "text/plain",
                disposition: "inline",
                filename: filename
    else
      render plain: "Script not found", status: :not_found
    end
  end
end
