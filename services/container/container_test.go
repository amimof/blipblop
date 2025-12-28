
			exp := tt.expect
			// Ignore timestamps
			res.Container.Meta.Created = nil
			res.Container.Meta.Updated = nil
			res.Container.Meta.Revision = 0
			if !proto.Equal(exp, res.Container) {
				t.Errorf("\ngot:\n%v\nwant:\n%v", res.Container, exp)
			}
		})
	}
}
