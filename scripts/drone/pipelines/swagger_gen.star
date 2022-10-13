load(
    'scripts/drone/steps/lib.star',
    'go_image',
)

load(
    'scripts/drone/utils/utils.star',
    'pipeline',
)

load(
    'scripts/drone/vault.star',
    'github_token',
    'from_secret',
)

def clone_pr_branch(edition, ver_mode):
    if edition in ['enterprise', 'enterprise2']:
        return None
    
    if ver_mode != "pr":
        return None

    committish = '${DRONE_SOURCE_BRANCH}'
    return {
        'name': 'clone-branch',
        'image': go_image,
        'commands': [
            'git clone "https://github.com/grafana/grafana.git"',
            'cd grafana',
            'git checkout {}'.format(committish),
        ],
    }

def swagger_gen_step(edition, ver_mode):
    if edition in ['enterprise', 'enterprise2']:
        return None
    
    if ver_mode != "pr":
        return None

    return {
        'name': 'swagger-gen',
        'image': go_image,
        'depends_on': [],
        'environment': {
            'GITHUB_TOKEN': from_secret(github_token),
        },
        'commands': [
            'go run ./grafana/pkg/api/swaggergen/main.go ${DRONE_SOURCE_BRANCH} ${DRONE_COMMIT_SHA} $${GITHUB_TOKEN}',
        ],
    }

def swagger_gen(trigger, edition, ver_mode):
	test_steps = [
		clone_pr_branch(edition=edition, ver_mode=ver_mode),
		swagger_gen_step(edition=edition, ver_mode=ver_mode)
	]

	p = pipeline(
		name='{}-swagger-gen'.format(ver_mode), edition=edition, trigger=trigger, services=[], steps=test_steps,
	)
	
	p['clone'] = {
            'disable': True,
        }

	return p
