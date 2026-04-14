module Api
  module V1
    class HeartbeatsController < BaseController
      # POST /api/v1/heartbeats
      # WakaTime-compatible single heartbeat
      def create
        heartbeat = build_heartbeat(heartbeat_params)

        if heartbeat.save
          TinybirdService.ingest_async(heartbeat)
          render json: { data: heartbeat_response(heartbeat), responses: [ [ 201, nil ] ] },
                 status: :created
        else
          render json: { errors: heartbeat.errors.full_messages }, status: :unprocessable_entity
        end
      end

      # POST /api/v1/heartbeats.bulk
      # WakaTime-compatible bulk heartbeat ingestion
      def bulk
        raw = request.body.read
        heartbeats_data = JSON.parse(raw)

        results = []
        heartbeats_data.each do |hb_data|
          heartbeat = build_heartbeat(ActionController::Parameters.new(hb_data).permit(allowed_heartbeat_params))
          if heartbeat.save
            TinybirdService.ingest_async(heartbeat)
            results << [ 201, nil ]
          else
            results << [ 400, heartbeat.errors.full_messages ]
          end
        end

        render json: { responses: results }, status: :created
      rescue JSON::ParserError
        render json: { error: "Invalid JSON" }, status: :bad_request
      end

      private

      def heartbeat_params
        params.permit(allowed_heartbeat_params)
      end

      def allowed_heartbeat_params
        %i[entity type language project branch time is_write lines_added lines_deleted
           operating_system machine agent_type tokens_used cost_usd metadata]
      end

      def build_heartbeat(attrs)
        project = find_or_create_project(attrs[:project])

        HeartbeatEvent.new(
          user: current_user,
          project: project,
          entity: attrs[:entity],
          entity_type: attrs[:type] || attrs[:entity_type] || "file",
          language: attrs[:language],
          branch: attrs[:branch],
          time: attrs[:time] || Time.now.to_f,
          is_write: attrs[:is_write] || false,
          lines_added: attrs[:lines_added],
          lines_deleted: attrs[:lines_deleted],
          operating_system: attrs[:operating_system],
          machine: attrs[:machine],
          agent_type: attrs[:agent_type] || "custom",
          tokens_used: attrs[:tokens_used],
          cost_usd: attrs[:cost_usd],
          metadata: attrs[:metadata] || {}
        )
      end

      def find_or_create_project(name)
        return nil if name.blank?
        current_user.projects.find_or_create_by!(name: name)
      rescue ActiveRecord::RecordInvalid
        current_user.projects.find_by(name: name)
      end

      def heartbeat_response(heartbeat)
        {
          id: heartbeat.id,
          entity: heartbeat.entity,
          type: heartbeat.entity_type,
          language: heartbeat.language,
          branch: heartbeat.branch,
          time: heartbeat.time,
          project: heartbeat.project&.name,
          agent_type: heartbeat.agent_type,
          created_at: heartbeat.created_at
        }
      end
    end
  end
end
